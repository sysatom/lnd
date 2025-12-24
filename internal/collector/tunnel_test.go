package collector

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sysatom/lnd/internal/config"
)

func TestTunnelCollector_HTTP_TCP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	target := ts.Listener.Addr().String()

	cfg := []config.TunnelConfig{
		{
			Name:      "Test HTTP",
			Target:    target,
			App:       "http",
			Transport: "tcp",
		},
	}

	c := NewTunnelCollector(cfg)
	results := c.Collect()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "OK" {
		t.Errorf("expected OK, got %s (err: %v)", results[0].Status, results[0].Error)
	}
}

func TestTunnelCollector_WS_TCP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "Not a websocket handshake", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer ts.Close()

	target := ts.Listener.Addr().String()

	cfg := []config.TunnelConfig{
		{
			Name:      "Test WS",
			Target:    target,
			App:       "ws",
			Transport: "tcp",
		},
	}

	c := NewTunnelCollector(cfg)
	results := c.Collect()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "OK" {
		t.Errorf("expected OK, got %s (err: %v)", results[0].Status, results[0].Error)
	}
}

// Simple SOCKS5 server for testing
func startSocks5Server(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go handleSocks5(conn)
		}
	}()

	return l.Addr().String()
}

func handleSocks5(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	// 1. Auth negotiation
	// Ver(1) | NMethods(1) | Methods(N)
	_, err := reader.ReadByte() // Ver
	if err != nil {
		return
	}
	nMethods, err := reader.ReadByte()
	if err != nil {
		return
	}
	_, err = reader.Discard(int(nMethods))
	if err != nil {
		return
	}

	// Response: Ver(1) | Method(1) (0x00 = No Auth)
	conn.Write([]byte{0x05, 0x00})

	// 2. Request
	// Ver(1) | Cmd(1) | Rsv(1) | Atyp(1) | DstAddr(...) | DstPort(2)
	_, err = reader.ReadByte() // Ver
	if err != nil {
		return
	}
	cmd, err := reader.ReadByte()
	if err != nil {
		return
	}
	_, err = reader.ReadByte() // Rsv
	if err != nil {
		return
	}
	atyp, err := reader.ReadByte()
	if err != nil {
		return
	}

	var host string
	switch atyp {
	case 1: // IPv4
		ip := make([]byte, 4)
		_, err = io.ReadFull(reader, ip)
		if err != nil {
			return
		}
		host = net.IP(ip).String()
	case 3: // Domain
		lenByte, err := reader.ReadByte()
		if err != nil {
			return
		}
		domain := make([]byte, int(lenByte))
		_, err = io.ReadFull(reader, domain)
		if err != nil {
			return
		}
		host = string(domain)
	case 4: // IPv6
		ip := make([]byte, 16)
		_, err = io.ReadFull(reader, ip)
		if err != nil {
			return
		}
		host = net.IP(ip).String()
	}

	portBuf := make([]byte, 2)
	_, err = io.ReadFull(reader, portBuf)
	if err != nil {
		return
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])

	targetAddr := fmt.Sprintf("%s:%d", host, port)

	if cmd == 1 { // CONNECT
		targetConn, err := net.Dial("tcp", targetAddr)
		if err != nil {
			// Reply failure
			conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		defer targetConn.Close()

		// Reply success
		// Ver | Rep | Rsv | Atyp | BndAddr | BndPort
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

		// Proxy
		go func() {
			io.Copy(targetConn, reader)
		}()

		io.Copy(conn, targetConn)
	}
}

func TestTunnelCollector_HTTP_SOCKS5(t *testing.T) {
	// Target Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	target := ts.Listener.Addr().String()

	// Proxy Server
	proxyAddr := startSocks5Server(t)

	cfg := []config.TunnelConfig{
		{
			Name:      "Test HTTP over SOCKS5",
			Target:    target,
			App:       "http",
			Transport: "socks5",
			Proxy:     proxyAddr,
		},
	}

	c := NewTunnelCollector(cfg)
	results := c.Collect()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "OK" {
		t.Errorf("expected OK, got %s (err: %v)", results[0].Status, results[0].Error)
	}
}

func TestTunnelCollector_SOCKS5_TCP(t *testing.T) {
	// Target Server (SOCKS5)
	target := startSocks5Server(t)

	cfg := []config.TunnelConfig{
		{
			Name:      "Test SOCKS5 over TCP",
			Target:    target,
			App:       "socks5",
			Transport: "tcp",
		},
	}

	c := NewTunnelCollector(cfg)
	results := c.Collect()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "OK" {
		t.Errorf("expected OK, got %s (err: %v)", results[0].Status, results[0].Error)
	}
}

func startHttpProxy(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go handleHttpProxy(conn)
		}
	}()

	return l.Addr().String()
}

func handleHttpProxy(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}

	if req.Method == "CONNECT" {
		targetConn, err := net.Dial("tcp", req.Host)
		if err != nil {
			fmt.Fprintf(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
			return
		}
		defer targetConn.Close()

		fmt.Fprintf(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")

		go io.Copy(targetConn, reader)
		io.Copy(conn, targetConn)
	}
}

func TestTunnelCollector_HTTP_HTTPProxy(t *testing.T) {
	// Target Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	target := ts.Listener.Addr().String()

	// Proxy Server
	proxyAddr := startHttpProxy(t)

	cfg := []config.TunnelConfig{
		{
			Name:      "Test HTTP over HTTP Proxy",
			Target:    target,
			App:       "http",
			Transport: "http",
			Proxy:     proxyAddr,
		},
	}

	c := NewTunnelCollector(cfg)
	results := c.Collect()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "OK" {
		t.Errorf("expected OK, got %s (err: %v)", results[0].Status, results[0].Error)
	}
}

func TestTunnelCollector_TLS_TCP(t *testing.T) {
	// Start TLS Server
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	target := ts.Listener.Addr().String()

	cfg := []config.TunnelConfig{
		{
			Name:      "Test TLS over TCP",
			Target:    target,
			App:       "tls",
			Transport: "tcp",
		},
	}

	c := NewTunnelCollector(cfg)
	results := c.Collect()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "OK" {
		t.Errorf("expected OK, got %s (err: %v)", results[0].Status, results[0].Error)
	}
}
