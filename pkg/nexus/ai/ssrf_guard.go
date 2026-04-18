// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// SSRF defence for the HTTPLLMClient and any other HelixQA HTTP
// client that talks to an operator-supplied endpoint. Rejects the
// usual SSRF pivots before the request leaves the process:
//
//   - loopback (127.0.0.0/8, ::1)
//   - link-local (169.254.0.0/16, fe80::/10) — blocks cloud
//     metadata endpoints (EC2 / GCE / Azure)
//   - RFC1918 private ranges (10/8, 172.16/12, 192.168/16)
//   - IPv6 unique local (fc00::/7) + site-local
//   - 0.0.0.0 / unspecified addresses
//   - any host that only resolves to a disallowed address family
//
// Operators that *do* run an internal-only LLM can opt-out with
// SetAllowPrivateNetworks(true) so the guard does not block their
// on-prem inference box. Public-internet endpoints stay safe by
// default.
//
// The security guidance ("SSRF Defense — ssrf_filter / ssrf-req-
// filter") recommends blocking these ranges out of the box, which
// is exactly what this guard does.

// SSRFGuardConfig tunes the guard. Zero value is safe: all private
// ranges rejected.
type SSRFGuardConfig struct {
	// AllowPrivateNetworks lets requests reach RFC1918 / loopback /
	// link-local destinations. Only flip this on when the operator
	// knows the endpoint is an on-prem LLM they trust.
	AllowPrivateNetworks bool

	// AllowedSchemes lists the URI schemes the guard accepts.
	// Empty = http + https only.
	AllowedSchemes []string

	// Resolver overrides net.DefaultResolver so tests can inject
	// deterministic DNS responses without hitting the network.
	Resolver Resolver
}

// Resolver is a narrow interface the guard uses to enumerate IPs
// for a hostname. *net.Resolver satisfies it via LookupIP.
type Resolver interface {
	LookupIP(network, host string) ([]net.IP, error)
}

type stdlibResolver struct{}

func (stdlibResolver) LookupIP(network, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(context.Background(), network, host)
}

// ErrSSRFBlocked is returned when the guard refuses a URL. Wrapped
// with a descriptive reason so operators can debug misconfigured
// endpoints.
var ErrSSRFBlocked = errors.New("ssrf blocked")

// ValidateURL parses target and runs every guard check. Returns
// ErrSSRFBlocked (wrapped) on rejection, nil on pass.
func ValidateURL(target string, cfg SSRFGuardConfig) error {
	if target == "" {
		return fmt.Errorf("%w: empty url", ErrSSRFBlocked)
	}
	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("%w: parse: %v", ErrSSRFBlocked, err)
	}
	if err := validateScheme(u.Scheme, cfg); err != nil {
		return err
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrSSRFBlocked)
	}
	// Block literal 0.0.0.0 / :: no matter what — these have no
	// legitimate remote use.
	if host == "0.0.0.0" || host == "::" {
		return fmt.Errorf("%w: unspecified address %q", ErrSSRFBlocked, host)
	}

	// Direct IP literal path: no DNS needed.
	if ip := net.ParseIP(host); ip != nil {
		return checkIP(ip, cfg)
	}

	// Hostname path: resolve, then ensure every returned IP is
	// allowed. Block on first hit so a hostname that points at
	// both a public + private IP is refused.
	resolver := cfg.Resolver
	if resolver == nil {
		resolver = stdlibResolver{}
	}
	ips, lookupErr := resolver.LookupIP("ip", host)
	if lookupErr != nil {
		return fmt.Errorf("%w: lookup %s: %v", ErrSSRFBlocked, host, lookupErr)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: host %q resolves to zero IPs", ErrSSRFBlocked, host)
	}
	for _, ip := range ips {
		if err := checkIP(ip, cfg); err != nil {
			return err
		}
	}
	return nil
}

func validateScheme(scheme string, cfg SSRFGuardConfig) error {
	scheme = strings.ToLower(scheme)
	allowed := cfg.AllowedSchemes
	if len(allowed) == 0 {
		allowed = []string{"http", "https"}
	}
	for _, s := range allowed {
		if scheme == strings.ToLower(s) {
			return nil
		}
	}
	return fmt.Errorf("%w: scheme %q not in allow list", ErrSSRFBlocked, scheme)
}

func checkIP(ip net.IP, cfg SSRFGuardConfig) error {
	if ip.IsUnspecified() {
		return fmt.Errorf("%w: unspecified address %s", ErrSSRFBlocked, ip)
	}
	if cfg.AllowPrivateNetworks {
		return nil
	}
	if ip.IsLoopback() {
		return fmt.Errorf("%w: loopback %s", ErrSSRFBlocked, ip)
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		// 169.254/16 + fe80::/10 — also catches the AWS / GCP
		// metadata IPs (169.254.169.254).
		return fmt.Errorf("%w: link-local %s", ErrSSRFBlocked, ip)
	}
	if ip.IsPrivate() {
		return fmt.Errorf("%w: private address %s", ErrSSRFBlocked, ip)
	}
	if isIPv6UniqueLocal(ip) {
		return fmt.Errorf("%w: ULA fc00::/7 %s", ErrSSRFBlocked, ip)
	}
	if ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return fmt.Errorf("%w: multicast %s", ErrSSRFBlocked, ip)
	}
	return nil
}

func isIPv6UniqueLocal(ip net.IP) bool {
	v6 := ip.To16()
	if v6 == nil || ip.To4() != nil {
		return false
	}
	return v6[0]&0xfe == 0xfc
}
