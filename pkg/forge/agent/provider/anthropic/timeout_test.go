package anthropic

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
)

// TestDefaultClientHasTimeout — New must never hand out a client that can
// hang forever (the Lumi 30-minute silent hang).
func TestDefaultClientHasTimeout(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if p.client.Timeout <= 0 {
		t.Fatal("default client has no timeout — a stalled connection would hang forever")
	}
}

// TestHangingServerReturnsOnDeadline proves a call against a server that
// never responds returns once the client's deadline passes, instead of
// hanging.
func TestHangingServerReturnsOnDeadline(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	// The handler blocks until released. Deferred LIFO: close(release) runs
	// before hang.Close(), so the handler can exit and Close doesn't wait
	// out its 10-minute patience on a connection we deliberately stalled.
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // never respond while the client is waiting
	}))
	defer hang.Close()
	defer close(release)

	p, err := NewWithClient(&http.Client{Timeout: 150 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	p.baseURL = hang.URL

	start := time.Now()
	_, err = p.CreateMessage(nil, nil, agentcfg.ModelConfig{Model: "claude-sonnet-5"}, "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error from a hanging server")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("call took %s to fail — deadline not enforced", elapsed)
	}
}
