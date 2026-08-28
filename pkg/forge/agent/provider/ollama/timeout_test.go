package ollama

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
)

func TestDefaultClientHasTimeout(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if p.client.Timeout <= 0 {
		t.Fatal("default client has no timeout — a wedged local server would hang forever")
	}
}

func TestHangingServerReturnsOnDeadline(t *testing.T) {
	// Deferred LIFO: close(release) runs before hang.Close(), so the stalled
	// handler can exit and Close doesn't wait on it.
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer hang.Close()
	defer close(release)

	p := NewWithClient(hang.URL, &http.Client{Timeout: 150 * time.Millisecond})

	start := time.Now()
	_, err := p.CreateMessage(nil, nil, agentcfg.ModelConfig{Model: "llama3.2"}, "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error from a hanging server")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("call took %s to fail — deadline not enforced", elapsed)
	}
}
