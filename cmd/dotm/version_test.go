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

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if output != "dotm dev\n" {
		t.Errorf("output = %q, want %q", output, "dotm dev\n")
	}
}
