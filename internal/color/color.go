package color

import (
	"fmt"
	"io"
	"os"
)

// ANSI color codes
const (
	ansiReset      = "\033[0m"
	ansiBold       = "\033[1m"
	ansiDim        = "\033[2m"
	ansiItalic     = "\033[3m"
	ansiUnderline  = "\033[4m"
	ansiBlack      = "\033[30m"
	ansiRed        = "\033[31m"
	ansiGreen      = "\033[32m"
	ansiYellow     = "\033[33m"
	ansiBlue       = "\033[34m"
	ansiMagenta    = "\033[35m"
	ansiCyan       = "\033[36m"
	ansiWhite      = "\033[37m"
	ansiBrightBlack   = "\033[90m"
	ansiBrightRed     = "\033[91m"
	ansiBrightGreen   = "\033[92m"
	ansiBrightYellow  = "\033[93m"
	ansiBrightBlue    = "\033[94m"
	ansiBrightMagenta = "\033[95m"
	ansiBrightCyan    = "\033[96m"
	ansiBrightWhite   = "\033[97m"
)

// IsColorEnabled checks if color output should be enabled (checks for TTY).
func IsColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	// Check if the file descriptor is a terminal
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// WithColor wraps a string with color codes if color is enabled.
func WithColor(w io.Writer, text string, colorCode string) string {
	if !IsColorEnabled(w) {
		return text
	}
	return fmt.Sprintf("%s%s%s", colorCode, text, ansiReset)
}

// Style helper functions
func Bold(w io.Writer, text string) string {
	return WithColor(w, text, ansiBold)
}

func BoldGreen(w io.Writer, text string) string {
	return WithColor(w, text, ansiBold+ansiGreen)
}

func BoldBlue(w io.Writer, text string) string {
	return WithColor(w, text, ansiBold+ansiBlue)
}

func BoldYellow(w io.Writer, text string) string {
	return WithColor(w, text, ansiBold+ansiYellow)
}

func BoldRed(w io.Writer, text string) string {
	return WithColor(w, text, ansiBold+ansiRed)
}

func Green(w io.Writer, text string) string {
	return WithColor(w, text, ansiGreen)
}

func Blue(w io.Writer, text string) string {
	return WithColor(w, text, ansiBlue)
}

func Yellow(w io.Writer, text string) string {
	return WithColor(w, text, ansiYellow)
}

func Red(w io.Writer, text string) string {
	return WithColor(w, text, ansiRed)
}

func Cyan(w io.Writer, text string) string {
	return WithColor(w, text, ansiCyan)
}

func Magenta(w io.Writer, text string) string {
	return WithColor(w, text, ansiMagenta)
}

func Dim(w io.Writer, text string) string {
	return WithColor(w, text, ansiDim)
}
