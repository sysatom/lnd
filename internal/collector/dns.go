package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type DNSRecordType string

const (
	RecordA     DNSRecordType = "A"
	RecordAAAA  DNSRecordType = "AAAA"
	RecordCNAME DNSRecordType = "CNAME"
	RecordMX    DNSRecordType = "MX"
	RecordTXT   DNSRecordType = "TXT"
	RecordNS    DNSRecordType = "NS"
	RecordPTR   DNSRecordType = "PTR"
	RecordSRV   DNSRecordType = "SRV"
	RecordCAA   DNSRecordType = "CAA"
)

type DNSProtocol string

const (
	ProtoUDP DNSProtocol = "UDP"
	ProtoTCP DNSProtocol = "TCP"
	ProtoDoT DNSProtocol = "DoT"
	ProtoDoH DNSProtocol = "DoH"
	ProtoDoQ DNSProtocol = "DoQ" // Placeholder, might require quic-go
)

type DNSServer struct {
	Name    string
	Address string // IP:Port or URL for DoH
	Proto   DNSProtocol
}

var DefaultDNSServers = []DNSServer{
	{Name: "System", Address: "", Proto: ProtoUDP},
	{Name: "Google", Address: "8.8.8.8:53", Proto: ProtoUDP},
	{Name: "Cloudflare", Address: "1.1.1.1:53", Proto: ProtoUDP},
	{Name: "AliDNS", Address: "223.5.5.5:53", Proto: ProtoUDP},
	{Name: "Custom", Address: "", Proto: ProtoDoT},
}

type DNSLookupResult struct {
	Records      []string
	Latency      time.Duration
	Server       string
	Protocol     DNSProtocol
	Error        error
	CertInfo     *CertInfo // For encrypted protocols
	ResponseCode string
}

type CertInfo struct {
	Subject     string
	Issuer      string
	NotBefore   time.Time
	NotAfter    time.Time
	CipherSuite uint16
	Version     uint16
	DNSNames    []string
}

type DNSCollector struct {
}

func NewDNSCollector() *DNSCollector {
	return &DNSCollector{}
}

func (c *DNSCollector) Lookup(ctx context.Context, domain string, recordType DNSRecordType, server DNSServer) DNSLookupResult {
	// Handle Reverse Lookup (PTR) automatically if domain looks like an IP
	if recordType == RecordPTR || isIP(domain) {
		recordType = RecordPTR
		var err error
		domain, err = dns.ReverseAddr(domain)
		if err != nil {
			return DNSLookupResult{Error: fmt.Errorf("invalid IP for reverse lookup: %v", err)}
		}
	}

	// Ensure domain ends with .
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}

	qType := dns.TypeA
	switch recordType {
	case RecordA:
		qType = dns.TypeA
	case RecordAAAA:
		qType = dns.TypeAAAA
	case RecordCNAME:
		qType = dns.TypeCNAME
	case RecordMX:
		qType = dns.TypeMX
	case RecordTXT:
		qType = dns.TypeTXT
	case RecordNS:
		qType = dns.TypeNS
	case RecordPTR:
		qType = dns.TypePTR
	case RecordSRV:
		qType = dns.TypeSRV
	case RecordCAA:
		qType = dns.TypeCAA
	}

	msg := new(dns.Msg)
	msg.SetQuestion(domain, qType)
	msg.RecursionDesired = true

	switch server.Proto {
	case ProtoDoH:
		return c.lookupDoH(ctx, msg, server)
	case ProtoDoT:
		return c.lookupDoT(ctx, msg, server)
	case ProtoDoQ:
		return DNSLookupResult{Error: fmt.Errorf("DoQ not implemented yet")}
	default: // UDP/TCP
		return c.lookupStandard(ctx, msg, server)
	}
}

func (c *DNSCollector) lookupStandard(ctx context.Context, msg *dns.Msg, server DNSServer) DNSLookupResult {
	client := new(dns.Client)
	client.Net = "udp"

	address := server.Address
	if server.Name == "System" {
		// Use local resolver logic or default to /etc/resolv.conf parsing
		// For simplicity in this tool, we might want to parse /etc/resolv.conf or just use a default
		// But miekg/dns requires an address.
		// Let's try to find system resolvers.
		config, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err == nil && len(config.Servers) > 0 {
			address = net.JoinHostPort(config.Servers[0], config.Port)
		} else {
			address = "8.8.8.8:53" // Fallback
		}
	}

	start := time.Now()
	r, _, err := client.ExchangeContext(ctx, msg, address)
	latency := time.Since(start)

	if err != nil {
		return DNSLookupResult{Error: err, Latency: latency, Server: address, Protocol: ProtoUDP}
	}

	return parseResponse(r, latency, address, ProtoUDP, nil)
}

func (c *DNSCollector) lookupDoT(ctx context.Context, msg *dns.Msg, server DNSServer) DNSLookupResult {
	client := new(dns.Client)
	client.Net = "tcp-tls"

	// For DoT, we usually need port 853. If port is 53, switch to 853.
	address := server.Address
	host, port, err := net.SplitHostPort(address)
	if err == nil {
		if port == "53" {
			address = net.JoinHostPort(host, "853")
		}
	} else {
		address = net.JoinHostPort(address, "853")
	}

	// We need to capture TLS info. miekg/dns Client doesn't expose the conn easily in Exchange.
	// We might need to dial manually.

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false, // Should verify for security
	}

	// Extract host for TLS verification
	tlsHost, _, _ := net.SplitHostPort(address)
	tlsConfig.ServerName = tlsHost

	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	if err != nil {
		return DNSLookupResult{Error: err, Latency: time.Since(start), Server: address, Protocol: ProtoDoT}
	}
	defer conn.Close()

	dnsConn := new(dns.Conn)
	dnsConn.Conn = conn

	if err := dnsConn.WriteMsg(msg); err != nil {
		return DNSLookupResult{Error: err, Latency: time.Since(start), Server: address, Protocol: ProtoDoT}
	}

	r, err := dnsConn.ReadMsg()
	latency := time.Since(start)
	if err != nil {
		return DNSLookupResult{Error: err, Latency: latency, Server: address, Protocol: ProtoDoT}
	}

	certInfo := getCertInfo(conn.ConnectionState())

	return parseResponse(r, latency, address, ProtoDoT, certInfo)
}

func (c *DNSCollector) lookupDoH(ctx context.Context, msg *dns.Msg, server DNSServer) DNSLookupResult {
	// Pack message
	packed, err := msg.Pack()
	if err != nil {
		return DNSLookupResult{Error: err}
	}

	// Determine URL
	url := server.Address
	if !strings.HasPrefix(url, "https://") {
		// Map common IPs to their DoH endpoints if not provided
		if strings.HasPrefix(url, "8.8.8.8") {
			url = "https://dns.google/dns-query"
		} else if strings.HasPrefix(url, "1.1.1.1") {
			url = "https://cloudflare-dns.com/dns-query"
		} else if strings.HasPrefix(url, "223.5.5.5") {
			url = "https://dns.alidns.com/dns-query"
		} else {
			// Fallback or error?
			url = "https://" + url + "/dns-query"
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(packed)))
	if err != nil {
		return DNSLookupResult{Error: err}
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	start := time.Now()
	// Custom transport to capture TLS info?
	// Standard http client doesn't easily expose TLS state of the connection used.
	// However, we can use httptrace or just inspect the response if we trust the client.
	// Actually, Response.TLS contains the connection state!

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return DNSLookupResult{Error: err, Latency: latency, Server: url, Protocol: ProtoDoH}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DNSLookupResult{Error: fmt.Errorf("DoH server returned %d", resp.StatusCode), Latency: latency, Server: url, Protocol: ProtoDoH}
	}

	// Read body
	buf := make([]byte, 65535)
	n, err := resp.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return DNSLookupResult{Error: err, Latency: latency, Server: url, Protocol: ProtoDoH}
	}

	r := new(dns.Msg)
	if err := r.Unpack(buf[:n]); err != nil {
		return DNSLookupResult{Error: err, Latency: latency, Server: url, Protocol: ProtoDoH}
	}

	var certInfo *CertInfo
	if resp.TLS != nil {
		certInfo = getCertInfo(*resp.TLS)
	}

	return parseResponse(r, latency, url, ProtoDoH, certInfo)
}

func parseResponse(r *dns.Msg, latency time.Duration, server string, proto DNSProtocol, cert *CertInfo) DNSLookupResult {
	res := DNSLookupResult{
		Latency:      latency,
		Server:       server,
		Protocol:     proto,
		CertInfo:     cert,
		ResponseCode: dns.RcodeToString[r.Rcode],
	}

	for _, ans := range r.Answer {
		// Format the answer nicely
		// ans.String() returns the full record string (e.g., "google.com. 300 IN A 1.2.3.4")
		// We might want to clean it up or just use it as is.
		res.Records = append(res.Records, strings.ReplaceAll(ans.String(), "\t", " "))
	}

	return res
}

func getCertInfo(state tls.ConnectionState) *CertInfo {
	if len(state.PeerCertificates) == 0 {
		return nil
	}
	cert := state.PeerCertificates[0]
	return &CertInfo{
		Subject:     cert.Subject.String(),
		Issuer:      cert.Issuer.String(),
		NotBefore:   cert.NotBefore,
		NotAfter:    cert.NotAfter,
		CipherSuite: state.CipherSuite,
		Version:     state.Version,
		DNSNames:    cert.DNSNames,
	}
}

func isIP(s string) bool {
	return net.ParseIP(s) != nil
}
