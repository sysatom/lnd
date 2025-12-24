package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type PublicIPInfo struct {
	IP       string
	Provider string
	Error    error
}

type PublicIPCollector struct {
	providers []string
}

func NewPublicIPCollector() *PublicIPCollector {
	return &PublicIPCollector{
		providers: []string{
			"https://api.ipify.org?format=text",
			"https://ifconfig.me/ip",
			"https://icanhazip.com",
			"https://checkip.amazonaws.com",
			"https://ipinfo.io/ip",
			"https://ipecho.net/plain",
			"https://ident.me",
			"https://whatismyip.akamai.com",
			"https://myexternalip.com/raw",
		},
	}
}

func (c *PublicIPCollector) Collect() PublicIPInfo {
	// Try providers sequentially until one works
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, url := range c.providers {
		ip, err := c.fetchIP(ctx, url)
		if err == nil && ip != "" {
			return PublicIPInfo{
				IP:       ip,
				Provider: url,
			}
		}
	}

	return PublicIPInfo{
		Error: fmt.Errorf("failed to fetch public IP from all providers"),
	}
}

func (c *PublicIPCollector) fetchIP(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "curl/7.68.0") // Some services block unknown UAs

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	// Basic validation
	if len(ip) == 0 || len(ip) > 45 { // IPv6 max length is 45
		return "", fmt.Errorf("invalid response length")
	}

	return ip, nil
}
