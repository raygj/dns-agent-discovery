package discovery

import (
	"fmt"
	"strings"
)

// AgentRecord holds DNS pointer-layer metadata for a discovered agent.
type AgentRecord struct {
	Capability string   `json:"capability"`
	URL        string   `json:"url"`
	Caps       []string `json:"caps"`
	Proto      string   `json:"proto"`
	SRVHost    string   `json:"srv_host,omitempty"`
	SRVPort    uint16   `json:"srv_port,omitempty"`
	SRVPriority uint16  `json:"srv_priority,omitempty"`
	SRVWeight   uint16  `json:"srv_weight,omitempty"`
}

// ParseTXTStrings extracts url, caps, and proto from SkyDNS-style TXT strings.
func ParseTXTStrings(txts []string) (url, proto string, caps []string, err error) {
	for _, txt := range txts {
		txt = strings.TrimSpace(txt)
		switch {
		case strings.HasPrefix(txt, "url="):
			url = strings.TrimPrefix(txt, "url=")
		case strings.HasPrefix(txt, "proto="):
			proto = strings.TrimPrefix(txt, "proto=")
		case strings.HasPrefix(txt, "caps="):
			raw := strings.TrimPrefix(txt, "caps=")
			if raw != "" {
				for _, c := range strings.Split(raw, ",") {
					c = strings.TrimSpace(c)
					if c != "" {
						caps = append(caps, c)
					}
				}
			}
		}
	}
	if url == "" {
		return "", "", nil, fmt.Errorf("TXT record missing url= field")
	}
	return url, proto, caps, nil
}

// FormatTXTStrings builds TXT strings for registration.
func FormatTXTStrings(url, proto string, caps []string) []string {
	txts := []string{
		"url=" + url,
		"proto=" + proto,
	}
	if len(caps) > 0 {
		txts = append(txts, "caps="+strings.Join(caps, ","))
	}
	return txts
}
