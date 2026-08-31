package harness

import (
	"context"
	"os"
	"strings"
	"testing"
)

// A sandboxed server is granted a set of capabilities and the ambient
// environment is not among them. This is the difference between extending the
// parent environment and replacing it.
func TestCleanEnvDoesNotInheritTheParentEnvironment(t *testing.T) {
	t.Setenv("TROVERY_TEST_AMBIENT_SECRET", "sk-should-not-be-inherited")

	script := `import fs from 'node:fs';
process.stdout.write(JSON.stringify(Object.keys(process.env).sort()) + "\n");`
	dir := t.TempDir()
	path := dir + "/env.mjs"
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("/usr/bin/env"); err != nil {
		t.Skip("no /usr/bin/env")
	}
	node, err := lookNode()
	if err != nil {
		t.Skip("node not installed")
	}

	t.Run("clean replaces", func(t *testing.T) {
		out := runCapture(t, context.Background(), node, []string{path}, dir,
			Options{Env: []string{"GRANTED=yes"}, CleanEnv: true})
		if strings.Contains(out, "TROVERY_TEST_AMBIENT_SECRET") {
			t.Errorf("the ambient environment leaked into a clean-env process:\n%s", out)
		}
		if !strings.Contains(out, "GRANTED") {
			t.Errorf("the granted variable did not arrive:\n%s", out)
		}
	})

	t.Run("default still extends", func(t *testing.T) {
		out := runCapture(t, context.Background(), node, []string{path}, dir,
			Options{Env: []string{"GRANTED=yes"}})
		if !strings.Contains(out, "TROVERY_TEST_AMBIENT_SECRET") {
			t.Errorf("the default stopped inheriting, which would break existing callers:\n%s", out)
		}
	})
}
