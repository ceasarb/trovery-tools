package adapters

import (
	"testing"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
)

func TestLumiAdapter_Name(t *testing.T) {
	a := &LumiAdapter{}
	if a.Name() != "lumi" {
		t.Errorf("Name() = %q, want lumi", a.Name())
	}
}

func TestLumiAdapter_ResolveCommand(t *testing.T) {
	a := &LumiAdapter{}
	cmd := a.ResolveCommand(config.ToolConfig{})
	if len(cmd) != 1 || cmd[0] != "lumi" {
		t.Errorf("ResolveCommand() = %v, want [lumi]", cmd)
	}
}

func TestLumiAdapter_InstallHint(t *testing.T) {
	a := &LumiAdapter{}
	if a.InstallHint() == "" {
		t.Error("InstallHint() should not be empty")
	}
}

func TestLumiAdapter_InRegistry(t *testing.T) {
	if _, ok := Registry["lumi"]; !ok {
		t.Error("lumi adapter missing from Registry")
	}
	if a := ResolveAdapter("lumi", nil); a == nil || a.Name() != "lumi" {
		t.Error("ResolveAdapter(lumi) did not return the lumi adapter")
	}
}
