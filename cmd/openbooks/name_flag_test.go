package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNameFlagIsNotGloballyRequired(t *testing.T) {
	flag := desktopCmd.PersistentFlags().Lookup("name")
	if flag == nil {
		t.Fatal("name flag not registered")
	}
	if _, required := flag.Annotations[cobra.BashCompOneRequiredFlag]; required {
		t.Fatal("name flag should not be globally required")
	}
}

func TestCLIModeRequiresName(t *testing.T) {
	originalFlags := globalFlags
	t.Cleanup(func() {
		globalFlags = originalFlags
	})
	globalFlags.UserName = "   "

	err := cliCmd.PersistentPreRunE(cliCmd, nil)
	if err == nil {
		t.Fatal("expected missing CLI username to return an error")
	}
	if !strings.Contains(err.Error(), "--name is required in cli mode") {
		t.Fatalf("error = %q, want missing --name error", err)
	}
}

func TestCLIModeAcceptsName(t *testing.T) {
	originalFlags := globalFlags
	originalConfig := cliConfig
	t.Cleanup(func() {
		globalFlags = originalFlags
		cliConfig = originalConfig
	})
	globalFlags.UserName = "test_user"

	err := cliCmd.PersistentPreRunE(cliCmd, nil)
	if err != nil {
		t.Fatalf("PersistentPreRunE() returned error: %v", err)
	}
	if cliConfig.UserName != "test_user" {
		t.Fatalf("cliConfig.UserName = %q, want %q", cliConfig.UserName, "test_user")
	}
}
