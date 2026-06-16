package discovery

import (
	"testing"
)

func TestParseTXTStrings(t *testing.T) {
	tests := []struct {
		name      string
		txts      []string
		wantURL   string
		wantProto string
		wantCaps  []string
		wantErr   bool
	}{
		{
			name: "full record",
			txts: []string{
				"url=https://mcp.internal/v1/agents/db-reader",
				"caps=sql,crypto",
				"proto=mcp",
			},
			wantURL:   "https://mcp.internal/v1/agents/db-reader",
			wantProto: "mcp",
			wantCaps:  []string{"sql", "crypto"},
		},
		{
			name:    "missing url",
			txts:    []string{"proto=mcp"},
			wantErr: true,
		},
		{
			name:    "url only",
			txts:    []string{"url=http://localhost:8080"},
			wantURL: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, proto, caps, err := ParseTXTStrings(tt.txts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if url != tt.wantURL {
				t.Errorf("url: got %q want %q", url, tt.wantURL)
			}
			if proto != tt.wantProto {
				t.Errorf("proto: got %q want %q", proto, tt.wantProto)
			}
			if len(caps) != len(tt.wantCaps) {
				t.Fatalf("caps: got %v want %v", caps, tt.wantCaps)
			}
			for i := range caps {
				if caps[i] != tt.wantCaps[i] {
					t.Errorf("caps[%d]: got %q want %q", i, caps[i], tt.wantCaps[i])
				}
			}
		})
	}
}

func TestFormatTXTStrings(t *testing.T) {
	txts := FormatTXTStrings("https://example.com", "mcp", []string{"a", "b"})
	if len(txts) != 3 {
		t.Fatalf("expected 3 txt strings, got %d", len(txts))
	}
	url, proto, caps, err := ParseTXTStrings(txts)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://example.com" || proto != "mcp" || len(caps) != 2 {
		t.Fatalf("roundtrip failed: url=%s proto=%s caps=%v", url, proto, caps)
	}
}
