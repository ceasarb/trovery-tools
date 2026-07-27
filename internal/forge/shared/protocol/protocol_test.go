package protocol

import (
	"encoding/json"
	"testing"
)

func TestToolCallParams_MetaMarshalsAsUnderscoreMeta(t *testing.T) {
	params := ToolCallParams{
		Name:      "gateway.read",
		Arguments: map[string]any{"id": "42"},
		Meta:      map[string]any{"tandem.on_behalf_of": "assertion"},
	}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	meta, ok := decoded["_meta"]
	if !ok {
		t.Fatalf("expected _meta field, got %s", data)
	}
	var metaMap map[string]any
	if err := json.Unmarshal(meta, &metaMap); err != nil {
		t.Fatal(err)
	}
	if metaMap["tandem.on_behalf_of"] != "assertion" {
		t.Fatalf("meta key missing/wrong: %v", metaMap)
	}

	// Identity must never leak into the model-visible arguments.
	var argMap map[string]any
	if err := json.Unmarshal(decoded["arguments"], &argMap); err != nil {
		t.Fatal(err)
	}
	if _, present := argMap["tandem.on_behalf_of"]; present {
		t.Fatal("assertion leaked into arguments")
	}
}

func TestToolCallParams_NoMetaOmitsField(t *testing.T) {
	data, err := json.Marshal(ToolCallParams{Name: "t"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["_meta"]; ok {
		t.Fatalf("expected _meta omitted when unset, got %s", data)
	}
}
