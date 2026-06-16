package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildTestBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "dad")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build dad: %v\n%s", err, out)
	}
	return bin
}

func TestCLIUsageAndErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI subprocess tests in short mode")
	}
	bin := buildTestBinary(t)

	t.Run("missing subcommand", func(t *testing.T) {
		cmd := exec.Command(bin)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected failure")
		}
	})

	t.Run("lookup missing capability", func(t *testing.T) {
		cmd := exec.Command(bin, "lookup")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected failure")
		}
		if !strings.Contains(stderr.String(), "usage: dad lookup") {
			t.Fatalf("stderr: %q", stderr.String())
		}
	})

	t.Run("unknown subcommand", func(t *testing.T) {
		cmd := exec.Command(bin, "nope")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected failure")
		}
		if !strings.Contains(stderr.String(), "unknown subcommand") {
			t.Fatalf("stderr: %q", stderr.String())
		}
	})
}
