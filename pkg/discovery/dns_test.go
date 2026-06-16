package discovery

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type dnsFixture struct {
	addr string
	srv  *dns.Server
}

func startDNSFixture(t *testing.T, handler dns.Handler) *dnsFixture {
	t.Helper()

	l, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	addr := l.LocalAddr().String()

	srv := &dns.Server{
		PacketConn: l,
		Handler:    handler,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.ActivateAndServe()
	}()

	t.Cleanup(func() {
		_ = srv.Shutdown()
		wg.Wait()
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("udp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return &dnsFixture{addr: addr, srv: srv}
}

func agentDNSHandler() dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		if len(r.Question) == 0 {
			m.SetRcode(r, dns.RcodeFormatError)
			_ = w.WriteMsg(m)
			return
		}

		q := r.Question[0]
		name := q.Name
		switch name {
		case "db-reader.agents.cluster.local.":
			switch q.Qtype {
			case dns.TypeTXT:
				m.Answer = append(m.Answer,
					&dns.TXT{
						Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 1},
						Txt: []string{"url=https://mcp.example.com/v1/agents/db-reader", "proto=mcp", "caps=sql,crypto"},
					},
				)
			case dns.TypeSRV:
				m.Answer = append(m.Answer,
					&dns.SRV{
						Hdr:      dns.RR_Header{Name: name, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 1},
						Priority: 0,
						Weight:   5,
						Port:     443,
						Target:   "mcp.example.com.",
					},
				)
			default:
				m.SetRcode(r, dns.RcodeNameError)
			}
		case "empty-answer.agents.cluster.local.":
			if q.Qtype == dns.TypeTXT {
				// NOERROR with no TXT records.
			} else {
				m.SetRcode(r, dns.RcodeNameError)
			}
		case "no-srv.agents.cluster.local.":
			if q.Qtype == dns.TypeTXT {
				m.Answer = append(m.Answer, &dns.TXT{
					Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 1},
					Txt: []string{"url=https://example.com"},
				})
			} else {
				m.SetRcode(r, dns.RcodeNameError)
			}
		default:
			m.SetRcode(r, dns.RcodeNameError)
		}
		_ = w.WriteMsg(m)
	})
}
