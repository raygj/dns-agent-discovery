package registration

import (
	"testing"
)

func TestReverseDomain(t *testing.T) {
	got := reverseDomain("db-reader.agents.cluster.local")
	want := "local/cluster/agents/db-reader"
	if got != want {
		t.Errorf("reverseDomain: got %q want %q", got, want)
	}
}

func TestBaseKey(t *testing.T) {
	s := &Store{cfg: Config{
		PathPrefix:    "/skydns",
		ClusterDomain: "cluster.local",
	}}
	got := s.baseKey("db-reader")
	want := "/skydns/local/cluster/agents/db-reader"
	if got != want {
		t.Errorf("baseKey: got %q want %q", got, want)
	}
}

func TestParseSRVFromURL(t *testing.T) {
	tests := []struct {
		url      string
		wantHost string
		wantPort uint16
		wantErr  bool
	}{
		{"https://mcp.internal/v1/agents/db-reader", "mcp.internal.", 443, false},
		{"http://localhost:8080/path", "localhost.", 8080, false},
		{"http://example.com", "example.com.", 80, false},
		{"ftp://example.com", "example.com.", 443, false},
		{"http:///nohost", "", 0, true},
		{"http://example.com:99999", "", 0, true},
		{"://bad", "", 0, true},
	}

	for _, tt := range tests {
		host, port, err := parseSRVFromURL(tt.url)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", tt.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.url, err)
			continue
		}
		if host != tt.wantHost || port != tt.wantPort {
			t.Errorf("%s: got host=%q port=%d want host=%q port=%d", tt.url, host, port, tt.wantHost, tt.wantPort)
		}
	}
}

func TestFormatTXTStringsRegistration(t *testing.T) {
	txts := formatTXTStrings("https://example.com", "mcp", []string{"sql", "crypto"})
	if len(txts) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(txts))
	}
	if txts[0] != "url=https://example.com" {
		t.Errorf("unexpected txt[0]: %s", txts[0])
	}
}
