package main

import (
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
