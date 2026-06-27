package color

import (
	"bytes"
	"os"
	"testing"
)

func TestIsColorEnabled(t *testing.T) {
	// Stdout is a file; check if it's a TTY.
	enabled := IsColorEnabled(os.Stdout)
	// Just verify it doesn't panic and returns a bool.
	_ = enabled
}

func TestIsColorEnabled_NonFile(t *testing.T) {
	// A bytes.Buffer is not an *os.File, so should return false.
	var buf bytes.Buffer
	if IsColorEnabled(&buf) {
		t.Error("expected IsColorEnabled to return false for non-file writer")
	}
}

func TestWithColor(t *testing.T) {
	// With a non-file writer, should return text unchanged.
	var buf bytes.Buffer
	result := WithColor(&buf, "hello", ansiBlue)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}

	// With stdout (a file), may or may not be colored depending on TTY.
	// Just verify it doesn't panic.
	_ = WithColor(os.Stdout, "test", ansiRed)
}

func TestBold(t *testing.T) {
	var buf bytes.Buffer
	result := Bold(&buf, "bold text")
	_ = result
}

func TestBoldGreen(t *testing.T) {
	var buf bytes.Buffer
	result := BoldGreen(&buf, "green text")
	_ = result
}

func TestBoldBlue(t *testing.T) {
	var buf bytes.Buffer
	result := BoldBlue(&buf, "blue text")
	_ = result
}

func TestBoldYellow(t *testing.T) {
	var buf bytes.Buffer
	result := BoldYellow(&buf, "yellow text")
	_ = result
}

func TestBoldRed(t *testing.T) {
	var buf bytes.Buffer
	result := BoldRed(&buf, "red text")
	_ = result
}

func TestGreen(t *testing.T) {
	var buf bytes.Buffer
	result := Green(&buf, "green text")
	_ = result
}

func TestBlue(t *testing.T) {
	var buf bytes.Buffer
	result := Blue(&buf, "blue text")
	_ = result
}

func TestYellow(t *testing.T) {
	var buf bytes.Buffer
	result := Yellow(&buf, "yellow text")
	_ = result
}

func TestRed(t *testing.T) {
	var buf bytes.Buffer
	result := Red(&buf, "red text")
	_ = result
}

func TestCyan(t *testing.T) {
	var buf bytes.Buffer
	result := Cyan(&buf, "cyan text")
	_ = result
}

func TestMagenta(t *testing.T) {
	var buf bytes.Buffer
	result := Magenta(&buf, "magenta text")
	_ = result
}

func TestDim(t *testing.T) {
	var buf bytes.Buffer
	result := Dim(&buf, "dim text")
	_ = result
}
