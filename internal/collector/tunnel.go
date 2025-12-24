package collector

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/pion/dtls/v3"
	"github.com/sysatom/lnd/internal/config"
	"golang.org/x/net/proxy"
)

type TunnelResult struct {
	Name      string
	App       string
	Transport string
	Target    string
	Status    string // "OK" or "Error"
	Latency   time.Duration
	Error     error
}

type TunnelCollector struct {
	Config []config.TunnelConfig
}

func NewTunnelCollector(cfg []config.TunnelConfig) *TunnelCollector {
	return &TunnelCollector{Config: cfg}
}

func (c *TunnelCollector) Collect() []TunnelResult {
	var results []TunnelResult
	for _, cfg := range c.Config {
		start := time.Now()
		err := c.testTunnel(cfg)
		latency := time.Since(start)

		status := "OK"
		if err != nil {
			status = "Error"
		}

		results = append(results, TunnelResult{
			Name:      cfg.Name,
			App:       cfg.App,
			Transport: cfg.Transport,
			Target:    cfg.Target,
			Status:    status,
			Latency:   latency,
			Error:     err,
		})
	}
	return results
}

func (c *TunnelCollector) testTunnel(cfg config.TunnelConfig) error {
	// 1. Establish Transport (Protocol B)
	conn, err := c.dialTransport(cfg)
	if err != nil {
		return fmt.Errorf("transport error: %w", err)
	}
	defer conn.Close()

	// 2. Perform Application Check (Protocol A)
	return c.checkApplication(conn, cfg)
}

func (c *TunnelCollector) dialTransport(cfg config.TunnelConfig) (net.Conn, error) {
	timeout := 5 * time.Second

	switch cfg.Transport {
	case "tcp":
		return net.DialTimeout("tcp", cfg.Target, timeout)
	case "udp":
		return net.DialTimeout("udp", cfg.Target, timeout)
	case "tls":
		// TLS over TCP
		dialer := &net.Dialer{Timeout: timeout}
		return tls.DialWithDialer(dialer, "tcp", cfg.Target, &tls.Config{
			InsecureSkipVerify: true, // For diagnostics, we might want to allow this or make it configurable
		})
	case "dtls":
		addr, err := net.ResolveUDPAddr("udp", cfg.Target)
		if err != nil {
			return nil, err
		}
		return dtls.Dial("udp", addr, &dtls.Config{
			InsecureSkipVerify: true,
		})
	case "socks5":
		if cfg.Proxy == "" {
			return nil, fmt.Errorf("proxy address required for socks5")
		}
		var auth *proxy.Auth
		if cfg.User != "" || cfg.Password != "" {
			auth = &proxy.Auth{
				User:     cfg.User,
				Password: cfg.Password,
			}
		}
		dialer, err := proxy.SOCKS5("tcp", cfg.Proxy, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}
		return dialer.Dial("tcp", cfg.Target)
	case "http":
		if cfg.Proxy == "" {
			return nil, fmt.Errorf("proxy address required for http proxy")
		}
		// Connect to Proxy
		proxyConn, err := net.DialTimeout("tcp", cfg.Proxy, timeout)
		if err != nil {
			return nil, err
		}
		// Send CONNECT
		// Handle Basic Auth if User/Password provided
		req, err := http.NewRequest("CONNECT", "http://"+cfg.Target, nil)
		if err != nil {
			proxyConn.Close()
			return nil, err
		}
		if cfg.User != "" || cfg.Password != "" {
			req.SetBasicAuth(cfg.User, cfg.Password)
		}

		// Write request manually to control the connection
		// CONNECT host:port HTTP/1.1
		// Host: host:port
		// Proxy-Authorization: Basic ...
		// \r\n

		// Using req.Write is easier but it writes absolute URI for CONNECT which is correct
		err = req.Write(proxyConn)
		if err != nil {
			proxyConn.Close()
			return nil, err
		}

		// Read Response
		resp, err := http.ReadResponse(bufio.NewReader(proxyConn), req)
		if err != nil {
			proxyConn.Close()
			return nil, err
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			proxyConn.Close()
			return nil, fmt.Errorf("http proxy connect failed: %s", resp.Status)
		}
		return proxyConn, nil
	default:
		// TODO: Add support for kcp (requires github.com/xtaci/kcp-go)
		return nil, fmt.Errorf("unsupported transport protocol: %s", cfg.Transport)
	}
}

func (c *TunnelCollector) checkApplication(conn net.Conn, cfg config.TunnelConfig) error {
	// Set a deadline for the application check
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	switch cfg.App {
	case "tcp", "udp":
		// Connection established is enough for basic check
		// Optionally send a ping if needed, but for now just return nil
		return nil
	case "http":
		// Send a simple HTTP GET request
		req, err := http.NewRequest("GET", "http://"+cfg.Target, nil)
		if err != nil {
			return err
		}

		// Create a custom transport that uses our existing connection
		// This is tricky because http.Transport expects to dial itself.
		// Instead, we can write the request directly to the connection.

		err = req.Write(conn)
		if err != nil {
			return err
		}

		// Read response
		resp, err := http.ReadResponse(bufio.NewReader(conn), req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return nil
		}
		return fmt.Errorf("http status: %s", resp.Status)

	case "ws":
		// Basic WebSocket Handshake
		// We can manually construct the upgrade request
		// GET / HTTP/1.1
		// Upgrade: websocket
		// Connection: Upgrade
		// Sec-WebSocket-Key: ...
		// Sec-WebSocket-Version: 13

		// For simplicity, let's just check if we can write and read something,
		// or use a library if we want full WS support.
		// Given the constraints, manual handshake is safer than adding heavy deps if not needed.

		// Minimal WS Handshake
		fmt.Fprintf(conn, "GET / HTTP/1.1\r\n")
		fmt.Fprintf(conn, "Host: %s\r\n", cfg.Target)
		fmt.Fprintf(conn, "Upgrade: websocket\r\n")
		fmt.Fprintf(conn, "Connection: Upgrade\r\n")
		fmt.Fprintf(conn, "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n")
		fmt.Fprintf(conn, "Sec-WebSocket-Version: 13\r\n")
		fmt.Fprintf(conn, "\r\n")

		// Read response
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			return err
		}
		if resp.StatusCode != 101 {
			return fmt.Errorf("websocket upgrade failed: %s", resp.Status)
		}
		return nil

	case "socks5":
		// Simple SOCKS5 Handshake Check
		// Client: Ver(5) | NMethods(1) | Methods(0x00)
		_, err := conn.Write([]byte{0x05, 0x01, 0x00})
		if err != nil {
			return err
		}

		buf := make([]byte, 2)
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			return err
		}

		if buf[0] != 0x05 {
			return fmt.Errorf("invalid socks version: %x", buf[0])
		}
		if buf[1] == 0xFF {
			return fmt.Errorf("socks5 no acceptable methods")
		}
		return nil

	case "tls":
		// Perform TLS Handshake
		host := cfg.Target
		if h, _, err := net.SplitHostPort(cfg.Target); err == nil {
			host = h
		}

		tlsConn := tls.Client(conn, &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		})
		// We rely on the underlying connection deadline
		return tlsConn.Handshake()

	default:
		// TODO: Add support for kcp (requires github.com/xtaci/kcp-go)
		return fmt.Errorf("unsupported application protocol: %s", cfg.App)
	}
}
