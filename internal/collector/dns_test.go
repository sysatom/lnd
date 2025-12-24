package collector

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDNSLookup_A(t *testing.T) {
	c := NewDNSCollector()
	server := DNSServer{
		Name:    "Google",
		Address: "8.8.8.8:53",
		Proto:   ProtoUDP,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := c.Lookup(ctx, "google.com", RecordA, server)
	if res.Error != nil {
		t.Fatalf("Lookup failed: %v", res.Error)
	}

	if len(res.Records) == 0 {
		t.Error("Expected records, got none")
	}

	foundIP := false
	for _, rec := range res.Records {
		if strings.Contains(rec, "IN A") {
			foundIP = true
			break
		}
	}
	if !foundIP {
		t.Error("Expected A record in results")
	}
}

func TestDNSLookup_DoH(t *testing.T) {
	c := NewDNSCollector()
	server := DNSServer{
		Name:    "Cloudflare",
		Address: "https://cloudflare-dns.com/dns-query",
		Proto:   ProtoDoH,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := c.Lookup(ctx, "example.com", RecordA, server)
	if res.Error != nil {
		t.Fatalf("DoH Lookup failed: %v", res.Error)
	}

	if len(res.Records) == 0 {
		t.Error("Expected records, got none")
	}

	if res.Protocol != ProtoDoH {
		t.Errorf("Expected protocol DoH, got %s", res.Protocol)
	}
}

func TestDNSLookup_DoT(t *testing.T) {
	c := NewDNSCollector()
	server := DNSServer{
		Name:    "Cloudflare",
		Address: "1.1.1.1:853",
		Proto:   ProtoDoT,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := c.Lookup(ctx, "example.com", RecordA, server)
	if res.Error != nil {
		t.Fatalf("DoT Lookup failed: %v", res.Error)
	}

	if len(res.Records) == 0 {
		t.Error("Expected records, got none")
	}

	if res.CertInfo == nil {
		t.Error("Expected CertInfo for DoT, got nil")
	}
}

func TestDNSLookup_Reverse(t *testing.T) {
	c := NewDNSCollector()
	server := DNSServer{
		Name:    "Google",
		Address: "8.8.8.8:53",
		Proto:   ProtoUDP,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 8.8.8.8 -> dns.google.
	res := c.Lookup(ctx, "8.8.8.8", RecordPTR, server)
	if res.Error != nil {
		t.Fatalf("Reverse Lookup failed: %v", res.Error)
	}

	found := false
	for _, rec := range res.Records {
		if strings.Contains(rec, "dns.google") {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Records: %v", res.Records)
		t.Error("Expected dns.google in PTR record")
	}
}

func TestIsIP(t *testing.T) {
	if !isIP("1.1.1.1") {
		t.Error("1.1.1.1 should be IP")
	}
	if !isIP("2001:4860:4860::8888") {
		t.Error("IPv6 should be IP")
	}
	if isIP("google.com") {
		t.Error("google.com should not be IP")
	}
}
