package validate

import "testing"

func TestSecretEchoCatchesAnInjectedValue(t *testing.T) {
	secrets := map[string]string{"SLEEPER_TOKEN": "s3cr3t-value-abcdef123456"}
	vs := SecretEcho("nfl_player_stats",
		`{"player":"Caleb Williams","debug":{"auth":"s3cr3t-value-abcdef123456"}}`, secrets)

	if len(vs) != 1 {
		t.Fatalf("violations = %d, want 1", len(vs))
	}
	if vs[0].Severity != SeverityError {
		t.Errorf("severity = %s, want error — a leaked credential is not a warning", vs[0].Severity)
	}
	if vs[0].Category != CategorySecurity {
		t.Errorf("category = %s, want security", vs[0].Category)
	}
}

func TestSecretEchoNamesTheCredentialNotTheValue(t *testing.T) {
	secrets := map[string]string{"SLEEPER_TOKEN": "s3cr3t-value-abcdef123456"}
	vs := SecretEcho("t", "here it is: s3cr3t-value-abcdef123456", secrets)
	if len(vs) != 1 {
		t.Fatalf("violations = %d, want 1", len(vs))
	}
	// The report is read by people and written to logs. Repeating the value in
	// the finding would leak it a second time, through the thing complaining
	// about the leak.
	if got := vs[0].Message; contains(got, "s3cr3t-value-abcdef123456") {
		t.Errorf("the finding repeats the secret: %q", got)
	}
	if !contains(vs[0].Message, "SLEEPER_TOKEN") {
		t.Errorf("the finding should name the credential, got %q", vs[0].Message)
	}
}

func TestSecretEchoIgnoresCleanResponses(t *testing.T) {
	secrets := map[string]string{"SLEEPER_TOKEN": "s3cr3t-value-abcdef123456"}
	if vs := SecretEcho("t", `{"player":"Caleb Williams","yards":210}`, secrets); len(vs) != 0 {
		t.Errorf("a clean response should pass, got %v", vs)
	}
}

func TestSecretEchoSkipsShortAndEmptyValues(t *testing.T) {
	// An unsupplied credential has no value to echo, and matching on "" would
	// report every response there is.
	if vs := SecretEcho("t", "anything at all", map[string]string{"A": ""}); len(vs) != 0 {
		t.Errorf("an empty value should match nothing, got %v", vs)
	}
	if vs := SecretEcho("t", "the cat sat", map[string]string{"A": "at"}); len(vs) != 0 {
		t.Errorf("a value too short to be a credential should be skipped, got %v", vs)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
