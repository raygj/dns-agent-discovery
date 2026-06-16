package discovery

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// Resolver performs DNS lookups against a configurable server.
type Resolver struct {
	Server  string
	Timeout time.Duration
}

// NewResolver returns a resolver with sensible defaults.
func NewResolver(server string, timeout time.Duration) *Resolver {
	if server == "" {
		server = "127.0.0.1:53"
	}
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	return &Resolver{Server: server, Timeout: timeout}
}

func (r *Resolver) exchange(name string, qtype uint16) (*dns.Msg, error) {
	client := &dns.Client{Timeout: r.Timeout}
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), qtype)
	resp, _, err := client.Exchange(msg, r.Server)
	if err != nil {
		return nil, fmt.Errorf("dns exchange for %s: %w", name, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("dns exchange for %s: empty response", name)
	}
	if resp.Rcode == dns.RcodeNameError {
		return nil, fmt.Errorf("NXDOMAIN: %s", name)
	}
	if resp.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("dns rcode %s for %s", dns.RcodeToString[resp.Rcode], name)
	}
	return resp, nil
}

// LookupTXT returns all TXT strings for name.
func (r *Resolver) LookupTXT(name string) ([]string, error) {
	resp, err := r.exchange(name, dns.TypeTXT)
	if err != nil {
		return nil, err
	}
	var txts []string
	for _, rr := range resp.Answer {
		if txt, ok := rr.(*dns.TXT); ok {
			txts = append(txts, txt.Txt...)
		}
	}
	if len(txts) == 0 {
		return nil, fmt.Errorf("no TXT records for %s", name)
	}
	return txts, nil
}

// LookupSRV returns the first SRV record for name.
func (r *Resolver) LookupSRV(name string) (host string, port uint16, priority, weight uint16, err error) {
	resp, err := r.exchange(name, dns.TypeSRV)
	if err != nil {
		return "", 0, 0, 0, err
	}
	for _, rr := range resp.Answer {
		if srv, ok := rr.(*dns.SRV); ok {
			host = strings.TrimSuffix(srv.Target, ".")
			return host, srv.Port, srv.Priority, srv.Weight, nil
		}
	}
	return "", 0, 0, 0, fmt.Errorf("no SRV records for %s", name)
}

// Ping checks DNS server reachability.
func (r *Resolver) Ping() error {
	conn, err := net.DialTimeout("udp", r.Server, r.Timeout)
	if err != nil {
		return fmt.Errorf("dns server unreachable at %s: %w", r.Server, err)
	}
	return conn.Close()
}
