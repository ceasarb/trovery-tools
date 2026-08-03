// Package delegation carries a per-request delegated-identity assertion from a
// served agent's invocation boundary to every tool call it makes.
//
// Forge is transport, not trust anchor (see ADR-008): the assertion is an opaque
// bearer string that forge neither mints, parses, nor verifies. It rides the
// request context to the MCP client, which attaches it to each tools/call under
// the protocol-standard _meta field. The resource owner — the tool server — is
// the only place that verifies it.
package delegation

import "context"

// MetaKey is the _meta field key under which the delegated-identity assertion is
// attached to an MCP tools/call. It is a wire-level interop convention: a tool
// server that wants delegation reads this key from _meta and verifies the value.
const MetaKey = "trove.on_behalf_of"

// ctxKey is an unexported context key type so no other package can collide with
// or overwrite the stored assertion.
type ctxKey struct{}

// WithOnBehalfOf returns a copy of ctx carrying the given delegated-identity
// assertion. An empty assertion returns ctx unchanged, so absent ⇒ no _meta ⇒
// today's behavior exactly.
func WithOnBehalfOf(ctx context.Context, assertion string) context.Context {
	if assertion == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, assertion)
}

// OnBehalfOf returns the delegated-identity assertion carried by ctx, if any.
// The boolean is false when no assertion is present.
func OnBehalfOf(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKey{}).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
