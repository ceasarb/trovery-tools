package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
)

func TestDefaultClientHasTimeout(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if p.client.Timeout <= 0 {
		t.Fatal("default client has no timeout — a stalled connection would hang forever")
	}
}

func TestHangingServerReturnsOnDeadline(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	// Deferred LIFO: close(release) runs before hang.Close(), so the stalled
	// handler can exit and Close doesn't wait on it.
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer hang.Close()
	defer close(release)

	p, err := NewWithClient(&http.Client{Timeout: 150 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	p.baseURL = hang.URL

	start := time.Now()
	_, err = p.CreateMessage(nil, nil, agentcfg.ModelConfig{Model: "gpt-5.2"}, "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error from a hanging server")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("call took %s to fail — deadline not enforced", elapsed)
	}
}
