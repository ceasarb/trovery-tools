package receipt

import (
	"strings"
	"testing"
	"time"
)

func sample(request string) Receipt {
	return Receipt{
		Source: "/tmp/lumi.db", RunID: 7,
		At:      time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC),
		Request: request, Steps: 2, Attendance: "attended",
		ContractMet: true,
		ForecastUSD: 0.0026, ActualUSD: 0.0008, OverheadUSD: 0.0001,
		PolicyVersion: "1",
	}
}

// TestMarkdownConfinesUntrustedText pins the WALK trust requirement: text the
// user (or model) authored appears only inside the fenced block, and cannot
// restructure the document with its own Markdown.
func TestMarkdownConfinesUntrustedText(t *testing.T) {
	hostile := "# Fake heading\n```\nRecorded by nobody\n```\n[link](https://example.com)"
	out := RenderMarkdown(sample(hostile))

	// The document's own headings are exactly the two we wrote — the injected
	// "# Fake heading" must not become one (i.e. never start a line outside
	// the fence; inside the fence it is data).
	var fenced bool
	var headings []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "````") {
			fenced = !fenced
			continue
		}
		if !fenced && strings.HasPrefix(line, "#") {
			headings = append(headings, line)
		}
	}
	if len(headings) != 2 {
		t.Fatalf("expected exactly 2 document headings, got %d: %q\n%s", len(headings), headings, out)
	}
	if fenced {
		t.Fatalf("fence not closed — injected fence marker escaped:\n%s", out)
	}

	// The injected triple-backtick fence must sit inside a longer fence.
	if !strings.Contains(out, "````\n"+"# Fake heading") {
		t.Fatalf("hostile text is not confined by a longer fence:\n%s", out)
	}
}

func TestMarkdownHonestSentence(t *testing.T) {
	out := RenderMarkdown(sample("summarize the meeting notes"))
	if !strings.Contains(out, HonestSentence) {
		t.Fatalf("markdown receipt is missing the honest sentence:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "independent") {
		t.Fatalf("markdown receipt overclaims with 'independent' (ADR-012):\n%s", out)
	}
	if !strings.Contains(out, "as given by the user") {
		t.Fatalf("request is not attributed as user input:\n%s", out)
	}
}

// TestMarkdownStripsInvisibleCharacters — zero-width and bidi-override
// characters are sanitized before the request is shown, same posture as the
// runtime's tool-result pipeline.
func TestMarkdownStripsInvisibleCharacters(t *testing.T) {
	out := RenderMarkdown(sample("pay​ the‮ invoice"))
	if strings.ContainsRune(out, '​') || strings.ContainsRune(out, '‮') {
		t.Fatalf("invisible characters survived sanitization:\n%q", out)
	}
	if !strings.Contains(out, "pay the invoice") {
		t.Fatalf("sanitized request text missing:\n%s", out)
	}
}
