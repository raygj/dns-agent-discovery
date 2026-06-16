package discovery

import (
	"fmt"
	"strings"
	"time"
)
type Client struct {
	Zone     string
	Resolver *Resolver
}

// NewClient creates a discovery client for agents.<cluster-domain>.
func NewClient(clusterDomain, dnsServer string, timeout time.Duration) *Client {
	zone := "agents." + strings.TrimSuffix(clusterDomain, ".")
	return &Client{
		Zone:     zone,
		Resolver: NewResolver(dnsServer, timeout),
	}
}

// Lookup resolves capability metadata from the agents sub-zone.
func (c *Client) Lookup(capability string) (*AgentRecord, error) {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return nil, fmt.Errorf("capability must not be empty")
	}
	name := capability + "." + c.Zone

	txts, err := c.Resolver.LookupTXT(name)
	if err != nil {
		return nil, err
	}
	url, proto, caps, err := ParseTXTStrings(txts)
	if err != nil {
		return nil, err
	}

	rec := &AgentRecord{
		Capability: capability,
		URL:        url,
		Caps:       caps,
		Proto:      proto,
	}

	host, port, pri, weight, err := c.Resolver.LookupSRV(name)
	if err == nil {
		rec.SRVHost = host
		rec.SRVPort = port
		rec.SRVPriority = pri
		rec.SRVWeight = weight
	}

	return rec, nil
}
