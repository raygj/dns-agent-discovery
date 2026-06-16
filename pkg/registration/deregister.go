package registration

import (
	"context"
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Deregister removes all TXT+SRV records for an agent capability.
func (s *Store) Deregister(ctx context.Context, capability string) error {
	if capability == "" {
		return fmt.Errorf("capability must not be empty")
	}
	base := s.baseKey(capability)
	if _, err := s.client.Delete(ctx, base, clientv3.WithPrefix()); err != nil {
		return fmt.Errorf("delete records: %w", err)
	}
	return nil
}
