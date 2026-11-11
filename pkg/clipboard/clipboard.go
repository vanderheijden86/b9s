// Package clipboard writes text through system utilities or the terminal.
package clipboard

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	system "github.com/atotto/clipboard"
	"golang.org/x/term"
)

// Mode selects the clipboard transport.
type Mode string

const (
	Auto   Mode = "auto"
	System Mode = "system"
	OSC52  Mode = "osc52"
)

// UnmarshalText rejects configuration values that cannot select a transport.
func (m *Mode) UnmarshalText(text []byte) error {
	mode := Mode(text)
	switch mode {
	case "":
		mode = Auto
	case Auto, System, OSC52:
	default:
		return fmt.Errorf("invalid clipboard mode %q: want auto, system or osc52", mode)
	}
	*m = mode
	return nil
}

// Write copies text using the requested mode. The zero mode uses Auto.
func Write(text string, mode Mode) error {
	w := writer{
		system: system.WriteAll,
		stdout: os.Stdout,
		tty:    term.IsTerminal(int(os.Stdout.Fd())),
		term:   os.Getenv("TERM"),
	}
	return w.write(text, mode)
}

type writer struct {
	system func(string) error
	stdout io.Writer
	tty    bool
	term   string
}

func (w writer) write(text string, mode Mode) error {
	switch mode {
	case System:
		return w.system(text)
	case OSC52:
		return w.writeOSC52(text)
	case "", Auto:
		if err := w.system(text); err != nil {
			if fallbackErr := w.writeOSC52(text); fallbackErr != nil {
				return fmt.Errorf("system clipboard: %w; OSC52 fallback: %w", err, fallbackErr)
			}
		}
		return nil
	default:
		return fmt.Errorf("invalid clipboard mode %q: want auto, system or osc52", mode)
	}
}

func (w writer) writeOSC52(text string) error {
	terminal := strings.ToLower(strings.TrimSpace(w.term))
	if !w.tty || terminal == "dumb" {
		return fmt.Errorf("OSC52 unavailable: stdout must be a tty and TERM must not be dumb")
	}
	const maxEncodedSize = 74994
	if size := base64.StdEncoding.EncodedLen(len(text)); size > maxEncodedSize {
		return fmt.Errorf("OSC52 payload exceeds encoded size limit (%d > %d)", size, maxEncodedSize)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	sequence := "\x1b]52;c;" + encoded + "\a"
	if strings.HasPrefix(terminal, "screen") {
		// Screen passes DCS contents to the outer terminal, with a small
		// per-sequence buffer that requires splitting long payloads.
		var chunks strings.Builder
		chunks.WriteString("\x1bP\x1b]52;c;")
		for len(encoded) > 76 {
			chunks.WriteString(encoded[:76])
			chunks.WriteString("\x1b\\\x1bP")
			encoded = encoded[76:]
		}
		chunks.WriteString(encoded)
		chunks.WriteString("\a\x1b\\")
		sequence = chunks.String()
	}
	n, err := w.stdout.Write([]byte(sequence))
	if err != nil {
		return fmt.Errorf("writing OSC52 clipboard: %w", err)
	}
	if n != len(sequence) {
		return fmt.Errorf("writing OSC52 clipboard: %w", io.ErrShortWrite)
	}
	return nil
}
