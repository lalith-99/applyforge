package jobs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var blockedOutboundCIDRs = mustParseCIDRs(
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
	"2001:db8::/32",
)

func mustParseCIDRs(values ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic(err)
		}
		out = append(out, network)
	}
	return out
}

func validatePublicHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("parse outbound URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("outbound URL must use http or https")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("outbound URL requires a hostname")
	}
	if parsed.User != nil {
		return nil, errors.New("outbound URL must not contain credentials")
	}
	if literal := net.ParseIP(parsed.Hostname()); literal != nil && !isPublicInternetIP(literal) {
		return nil, errors.New("outbound URL resolves to a non-public IP")
	}
	return parsed, nil
}

func isPublicInternetIP(ip net.IP) bool {
	if ip == nil ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() {
		return false
	}
	for _, network := range blockedOutboundCIDRs {
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

func newPublicHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxIdleConnsPerHost = 4
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("split outbound address: %w", err)
		}

		if literal := net.ParseIP(host); literal != nil {
			if !isPublicInternetIP(literal) {
				return nil, errors.New("outbound connection to non-public IP is blocked")
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(literal.String(), port))
		}

		candidates, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve outbound host %s: %w", host, err)
		}
		for _, candidate := range candidates {
			if !isPublicInternetIP(candidate.IP) {
				continue
			}
			conn, dialErr := dialer.DialContext(
				ctx,
				network,
				net.JoinHostPort(candidate.IP.String(), port),
			)
			if dialErr == nil {
				return conn, nil
			}
		}
		return nil, fmt.Errorf("outbound host %s has no reachable public IP", host)
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many outbound redirects")
			}
			if _, err := validatePublicHTTPURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}
