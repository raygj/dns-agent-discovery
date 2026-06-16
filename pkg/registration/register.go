package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RegisterOptions holds per-agent registration parameters.
type RegisterOptions struct {
	Capability string
	URL        string
	Caps       []string
	Proto      string
	SRVHost    string
	SRVPort    uint16
}

// Register writes TXT+SRV records for an agent capability.
func (s *Store) Register(ctx context.Context, opts RegisterOptions) error {
	opts.Capability = strings.TrimSpace(opts.Capability)
	if opts.Capability == "" {
		return fmt.Errorf("capability must not be empty")
	}
	if strings.TrimSpace(opts.URL) == "" {
		return fmt.Errorf("url must not be empty")
	}
	if opts.Proto == "" {
		opts.Proto = "mcp"
	}

	srvHost := opts.SRVHost
	srvPort := opts.SRVPort
	if srvHost == "" || srvPort == 0 {
		host, port, err := parseSRVFromURL(opts.URL)
		if err != nil {
			return err
		}
		if srvHost == "" {
			srvHost = host
		}
		if srvPort == 0 {
			srvPort = port
		}
	}
	if !strings.HasSuffix(srvHost, ".") {
		srvHost += "."
	}

	base := s.baseKey(opts.Capability)
	srvKey := base + "/srv"

	srvJSON, err := json.Marshal(srvValue{
		Host:     srvHost,
		Port:     srvPort,
		Priority: 0,
		Weight:   5,
		TTL:      s.cfg.TTL,
	})
	if err != nil {
		return fmt.Errorf("marshal srv: %w", err)
	}

	if _, err := s.client.Put(ctx, srvKey, string(srvJSON)); err != nil {
		return fmt.Errorf("write srv record: %w", err)
	}

	txts := formatTXTStrings(opts.URL, opts.Proto, opts.Caps)
	for i, txt := range txts {
		txtKey := fmt.Sprintf("%s/txt%d", base, i)
		txtJSON, err := json.Marshal(txtValue{Text: txt, TTL: s.cfg.TTL})
		if err != nil {
			return fmt.Errorf("marshal txt: %w", err)
		}
		if _, err := s.client.Put(ctx, txtKey, string(txtJSON)); err != nil {
			return fmt.Errorf("write txt record: %w", err)
		}
	}
	return nil
}
