package registration

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const defaultPathPrefix = "/skydns"

// Config holds etcd registration settings.
type Config struct {
	Endpoints     []string
	PathPrefix    string
	ClusterDomain string
	TTL           int // seconds
	Username      string
	Password      string
	DialTimeout   time.Duration
}

// Store manages agent DNS records in etcd (SkyDNS format for CoreDNS).
type Store struct {
	client *clientv3.Client
	cfg    Config
}

// Entry represents a registered agent in etcd.
type Entry struct {
	Capability string   `json:"capability"`
	URL        string   `json:"url"`
	Caps       []string `json:"caps"`
	Proto      string   `json:"proto"`
	SRVHost    string   `json:"srv_host,omitempty"`
	SRVPort    uint16   `json:"srv_port,omitempty"`
}

// NewStore connects to etcd with the given config.
func NewStore(cfg Config) (*Store, error) {
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"http://127.0.0.1:2379"}
	}
	if cfg.PathPrefix == "" {
		cfg.PathPrefix = defaultPathPrefix
	}
	if cfg.ClusterDomain == "" {
		cfg.ClusterDomain = "cluster.local"
	}
	if cfg.TTL == 0 {
		cfg.TTL = 1
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("connect etcd: %w", err)
	}

	return &Store{client: client, cfg: cfg}, nil
}

// Close closes the etcd client.
func (s *Store) Close() error {
	return s.client.Close()
}

// Ping checks etcd connectivity.
func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.DialTimeout)
	defer cancel()
	_, err := s.client.Status(ctx, s.cfg.Endpoints[0])
	if err != nil {
		return fmt.Errorf("etcd unreachable: %w", err)
	}
	return nil
}

func reverseDomain(fqdn string) string {
	fqdn = strings.TrimSuffix(fqdn, ".")
	parts := strings.Split(fqdn, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, "/")
}

func (s *Store) baseKey(capability string) string {
	fqdn := capability + ".agents." + strings.TrimSuffix(s.cfg.ClusterDomain, ".")
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.PathPrefix, "/"), reverseDomain(fqdn))
}

type srvValue struct {
	Host     string `json:"host"`
	Port     uint16 `json:"port"`
	Priority uint16 `json:"priority"`
	Weight   uint16 `json:"weight"`
	TTL      int    `json:"ttl"`
}

type txtValue struct {
	Text string `json:"text"`
	TTL  int    `json:"ttl"`
}

func parseSRVFromURL(rawURL string) (host string, port uint16, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, fmt.Errorf("parse url: %w", err)
	}
	host = u.Hostname()
	if host == "" {
		return "", 0, fmt.Errorf("url missing host")
	}
	if !strings.HasSuffix(host, ".") {
		host += "."
	}
	switch p := u.Port(); p {
	case "":
		switch u.Scheme {
		case "https":
			port = 443
		case "http":
			port = 80
		default:
			port = 443
		}
	default:
		var n int
		_, err = fmt.Sscanf(p, "%d", &n)
		if err != nil || n <= 0 || n > 65535 {
			return "", 0, fmt.Errorf("invalid port in url: %s", p)
		}
		port = uint16(n)
	}
	return host, port, nil
}

func formatTXTStrings(agentURL, proto string, caps []string) []string {
	txts := []string{"url=" + agentURL, "proto=" + proto}
	if len(caps) > 0 {
		txts = append(txts, "caps="+strings.Join(caps, ","))
	}
	return txts
}
