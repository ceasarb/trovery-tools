package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/delegation"
)

// newPipedClient wires a Client to an in-memory fake MCP server so CallTool
// exercises the real send/marshal path without spawning a subprocess. It returns
// the client and a channel that receives the raw tools/call params the "server"
// observed.
func newPipedClient(t *testing.T) (*Client, <-chan map[string]json.RawMessage) {
	t.Helper()

	srvReads, cliWrites := io.Pipe() // client stdin -> server
	cliReads, srvWrites := io.Pipe() // server -> client stdout

	c := &Client{
		stdin:  cliWrites,
		reader: bufio.NewReader(cliReads),
		nextID: 1,
	}

	observed := make(chan map[string]json.RawMessage, 1)

	go func() {
		serverReader := bufio.NewReader(srvReads)
		line, err := serverReader.ReadString('\n')
		if err != nil {
			return
		}

		var req struct {
			ID     int                        `json:"id"`
			Params map[string]json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			return
		}
		observed <- req.Params

		resp := `{"jsonrpc":"2.0","id":` +
			itoa(req.ID) +
			`,"result":{"content":[{"type":"text","text":"ok"}]}}` + "\n"
		io.WriteString(srvWrites, resp)
	}()

	return c, observed
}

func itoa(i int) string {
	return string(rune('0' + i)) // ids are single-digit in these tests
}

func TestCallTool_AttachesOnBehalfOfAsMeta(t *testing.T) {
	c, observed := newPipedClient(t)

	ctx := delegation.WithOnBehalfOf(context.Background(), "signed-assertion")
	_, _, err := c.CallTool(ctx, "gateway.read", map[string]any{"id": "42"})
	if err != nil {
		t.Fatal(err)
	}

	params := <-observed

	meta, ok := params["_meta"]
	if !ok {
		t.Fatalf("expected _meta in tools/call params, got %v", params)
	}
	var metaMap map[string]any
	if err := json.Unmarshal(meta, &metaMap); err != nil {
		t.Fatal(err)
	}
	if metaMap[delegation.MetaKey] != "signed-assertion" {
		t.Fatalf("assertion not propagated under %s: %v", delegation.MetaKey, metaMap)
	}

	// The assertion must not appear in the model-visible arguments.
	var argMap map[string]any
	if err := json.Unmarshal(params["arguments"], &argMap); err != nil {
		t.Fatal(err)
	}
	if _, present := argMap[delegation.MetaKey]; present {
		t.Fatal("assertion leaked into arguments")
	}
}

func TestCallTool_NoAssertionNoMeta(t *testing.T) {
	c, observed := newPipedClient(t)

	_, _, err := c.CallTool(context.Background(), "gateway.read", map[string]any{"id": "42"})
	if err != nil {
		t.Fatal(err)
	}

	params := <-observed
	if _, ok := params["_meta"]; ok {
		t.Fatalf("expected no _meta when no assertion present, got %v", params)
	}
}
