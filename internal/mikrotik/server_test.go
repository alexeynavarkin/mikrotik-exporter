package mikrotik

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-routeros/routeros/v3/proto"
	"go.uber.org/zap"
)

// fakeServer speaks the RouterOS API: it accepts any login and answers every
// command with a single !re sentence, unless hang or drop say otherwise.
type fakeServer struct {
	addr        string
	connections atomic.Int32
	hang        atomic.Bool // stop answering commands (login still works)
	dropNext    atomic.Bool // close the connection on the next command
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })

	s := &fakeServer{addr: l.Addr().String()}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			s.connections.Add(1)
			go s.serve(conn)
		}
	}()
	return s
}

func (s *fakeServer) serve(conn net.Conn) {
	defer conn.Close()
	r, w := proto.NewReader(conn), proto.NewWriter(conn)
	for {
		sen, err := r.ReadSentence()
		if err != nil {
			return
		}
		if sen.Word != "/login" {
			if s.dropNext.CompareAndSwap(true, false) {
				return
			}
			if s.hang.Load() {
				continue
			}
			w.BeginSentence()
			w.WriteWord("!re")
			w.WriteWord("=cmd=" + sen.Word)
			_ = w.EndSentence()
		}
		w.BeginSentence()
		w.WriteWord("!done")
		_ = w.EndSentence()
	}
}

func newTestClient(addr string) Client {
	return NewRetryClient(RetryClientConfig{Address: addr, RetryCount: 2, PlainText: true}, zap.NewNop())
}

func TestRetryClientManyFastReplies(t *testing.T) {
	server := newFakeServer(t)
	client := newTestClient(server.addr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < 2000; i++ {
		res, err := client.RunContext(ctx, "/system/resource/print")
		if err != nil {
			t.Fatalf("command %d failed: %v", i, err)
		}
		if len(res.Re) != 1 || res.Re[0].Map["cmd"] != "/system/resource/print" {
			t.Fatalf("command %d: unexpected reply %v", i, res)
		}
	}
	if n := server.connections.Load(); n != 1 {
		t.Errorf("expected a single connection, got %d", n)
	}
}

func TestRetryClientHonorsTimeout(t *testing.T) {
	server := newFakeServer(t)
	client := newTestClient(server.addr)
	server.hang.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.RunContext(ctx, "/system/resource/print")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("command took %v, timeout not applied", elapsed)
	}

	// The device recovered: the next command must reconnect and succeed.
	server.hang.Store(false)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if _, err := client.RunContext(ctx2, "/system/resource/print"); err != nil {
		t.Fatalf("command after timeout failed: %v", err)
	}
}

func TestRetryClientReconnectsAfterConnectionLoss(t *testing.T) {
	server := newFakeServer(t)
	client := newTestClient(server.addr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := client.RunContext(ctx, "/system/resource/print"); err != nil {
		t.Fatal(err)
	}

	server.dropNext.Store(true)
	if _, err := client.RunContext(ctx, "/system/resource/print"); err != nil {
		t.Fatalf("command was not retried after connection loss: %v", err)
	}
	if n := server.connections.Load(); n != 2 {
		t.Errorf("expected a reconnect, got %d connections", n)
	}
}
