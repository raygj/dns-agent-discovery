package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// List returns all registered agent entries under the agents sub-zone.
func (s *Store) List(ctx context.Context) ([]Entry, error) {
	prefix := fmt.Sprintf("%s/%s/",
		strings.TrimSuffix(s.cfg.PathPrefix, "/"),
		reverseDomain("agents."+strings.TrimSuffix(s.cfg.ClusterDomain, ".")))

	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}

	type partial struct {
		capability string
		txts       []string
		srv        *srvValue
	}

	byCap := map[string]*partial{}
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		rel := strings.TrimPrefix(key, prefix)
		parts := strings.Split(rel, "/")
		if len(parts) < 2 {
			continue
		}
		capability := parts[0]
		recordKey := parts[1]
		p, ok := byCap[capability]
		if !ok {
			p = &partial{capability: capability}
			byCap[capability] = p
		}
		switch {
		case recordKey == "srv":
			var v srvValue
			if err := json.Unmarshal(kv.Value, &v); err == nil {
				p.srv = &v
			}
		case strings.HasPrefix(recordKey, "txt"):
			var v txtValue
			if err := json.Unmarshal(kv.Value, &v); err == nil && v.Text != "" {
				p.txts = append(p.txts, v.Text)
			}
		}
	}

	var entries []Entry
	for cap, p := range byCap {
		e := Entry{Capability: cap}
		for _, t := range p.txts {
			switch {
			case strings.HasPrefix(t, "url="):
				e.URL = strings.TrimPrefix(t, "url=")
			case strings.HasPrefix(t, "proto="):
				e.Proto = strings.TrimPrefix(t, "proto=")
			case strings.HasPrefix(t, "caps="):
				raw := strings.TrimPrefix(t, "caps=")
				if raw != "" {
					e.Caps = strings.Split(raw, ",")
				}
			}
		}
		if p.srv != nil {
			e.SRVHost = strings.TrimSuffix(p.srv.Host, ".")
			e.SRVPort = p.srv.Port
		}
		if e.URL != "" {
			entries = append(entries, e)
		}
	}
	return entries, nil
}
