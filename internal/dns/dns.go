// Package dns contains provider operations. Authentication and domain persistence
// belong to the caller; no provider operation activates an artifact domain.
package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
)

type Record struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var ErrCollision = errors.New("DNS record already exists; Folio will not overwrite it")
var ErrNotOwned = errors.New("DNS record does not match Folio ownership")
var ErrUnsupported = errors.New("provider or template unsupported; configure DNS manually")

type Resolver interface {
	LookupTXT(context.Context, string) ([]string, error)
	LookupCNAME(context.Context, string) (string, error)
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

func canonical(s string) string { return strings.ToLower(strings.TrimSuffix(s, ".")) }
func ValidateHostname(s string) error {
	s = canonical(s)
	if len(s) == 0 || len(s) > 253 || net.ParseIP(s) != nil {
		return errors.New("invalid DNS hostname")
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return errors.New("hostname must be fully qualified")
	}
	for _, l := range labels {
		if len(l) == 0 || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return errors.New("invalid DNS hostname")
		}
		for _, c := range l {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return errors.New("invalid DNS hostname")
			}
		}
	}
	return nil
}
func DesiredRecords(hostname, challenge, target string) []Record {
	typ := "CNAME"
	if ip := net.ParseIP(target); ip != nil {
		typ = "AAAA"
		if ip.To4() != nil {
			typ = "A"
		}
	}
	return []Record{{"TXT", "_folio." + canonical(hostname), challenge}, {typ, canonical(hostname), strings.TrimSuffix(target, ".")}}
}
func Verify(ctx context.Context, hostname, challenge, target string) (bool, error) {
	return VerifyWithResolver(ctx, net.DefaultResolver, hostname, challenge, target)
}
func VerifyWithResolver(ctx context.Context, r Resolver, hostname, challenge, target string) (bool, error) {
	if err := ValidateHostname(hostname); err != nil {
		return false, err
	}
	if challenge == "" || target == "" {
		return false, errors.New("ownership challenge and target are required")
	}
	if r == nil {
		r = net.DefaultResolver
	}
	owned, err := VerifyOwnershipWithResolver(ctx, r, hostname, challenge)
	if err != nil || !owned {
		return false, err
	}
	if ip := net.ParseIP(target); ip != nil {
		ips, err := r.LookupIPAddr(ctx, hostname)
		if err != nil {
			return false, fmt.Errorf("routing address lookup: %w", err)
		}
		for _, got := range ips {
			if got.IP.Equal(ip) {
				return true, nil
			}
		}
		return false, nil
	}
	if err := ValidateHostname(target); err != nil {
		return false, err
	}
	cname, err := r.LookupCNAME(ctx, hostname)
	if err != nil {
		return false, fmt.Errorf("routing CNAME lookup: %w", err)
	}
	return canonical(cname) == canonical(target), nil
}

// VerifyOwnership checks the exact TXT challenge independently of proxy DNS.
func VerifyOwnership(ctx context.Context, hostname, challenge string) (bool, error) {
	return VerifyOwnershipWithResolver(ctx, net.DefaultResolver, hostname, challenge)
}
func VerifyOwnershipWithResolver(ctx context.Context, r Resolver, hostname, challenge string) (bool, error) {
	if err := ValidateHostname(hostname); err != nil {
		return false, err
	}
	if challenge == "" {
		return false, errors.New("ownership challenge is required")
	}
	if r == nil {
		r = net.DefaultResolver
	}
	rows, err := r.LookupTXT(ctx, "_folio."+canonical(hostname))
	if err != nil {
		return false, fmt.Errorf("ownership TXT lookup: %w", err)
	}
	for _, row := range rows {
		if row == challenge {
			return true, nil
		}
	}
	return false, nil
}
