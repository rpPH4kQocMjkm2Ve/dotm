package main

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestCmdVersion(t *testing.T) {
	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdVersion()

	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	output := buf.String()

	if output != "dotm dev\n" {
		t.Errorf("output = %q, want %q", output, "dotm dev\n")
	}
}
