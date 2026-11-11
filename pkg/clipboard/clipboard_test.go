package clipboard

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

type recordingOutput struct {
	bytes.Buffer
	calls int
	err   error
	short bool
}

func (o *recordingOutput) Write(p []byte) (int, error) {
	o.calls++
	if o.err != nil {
		return 0, o.err
	}
	if o.short {
		return len(p) - 1, nil
	}
	return o.Buffer.Write(p)
}

func TestWriteModes(t *testing.T) {
	systemErr := errors.New("system clipboard unavailable")
	for _, tc := range []struct {
		name        string
		mode        Mode
		systemErr   error
		systemCalls int
		outputCalls int
		wantErr     bool
	}{
		{name: "auto system succeeds", mode: Auto, systemCalls: 1},
		{name: "auto system fails", mode: Auto, systemErr: systemErr, systemCalls: 1, outputCalls: 1},
		{name: "zero mode defaults to auto", systemErr: systemErr, systemCalls: 1, outputCalls: 1},
		{name: "system succeeds", mode: System, systemCalls: 1},
		{name: "system failure never falls back", mode: System, systemErr: systemErr, systemCalls: 1, wantErr: true},
		{name: "osc52 bypasses system", mode: OSC52, outputCalls: 1},
		{name: "invalid mode", mode: "unknown", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out recordingOutput
			calls := 0
			w := writer{
				system: func(text string) error {
					calls++
					if text != "hello" {
						t.Fatalf("system received %q", text)
					}
					return tc.systemErr
				},
				stdout: &out, tty: true, term: "xterm-256color",
			}
			err := w.write("hello", tc.mode)
			if (err != nil) != tc.wantErr || calls != tc.systemCalls || out.calls != tc.outputCalls {
				t.Fatalf("err=%v, system calls=%d, output calls=%d", err, calls, out.calls)
			}
			if tc.outputCalls > 0 && out.String() != "\x1b]52;c;aGVsbG8=\a" {
				t.Fatalf("OSC52 output = %q", out.String())
			}
			if tc.mode == System && tc.systemErr != nil && !errors.Is(err, systemErr) {
				t.Fatalf("system error lost: %v", err)
			}
		})
	}
}

func TestOSC52RequiresTerminal(t *testing.T) {
	for _, tc := range []struct {
		name string
		tty  bool
		term string
	}{
		{name: "redirected stdout", term: "xterm"},
		{name: "dumb terminal", tty: true, term: "dumb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out recordingOutput
			w := writer{stdout: &out, tty: tc.tty, term: tc.term}
			if err := w.write("hello", OSC52); err == nil || out.calls != 0 {
				t.Fatalf("err=%v, output calls=%d, want refusal without output", err, out.calls)
			}
		})
	}
}

func TestAutoPreservesSystemAndTerminalErrors(t *testing.T) {
	systemErr := errors.New("utility failed")
	w := writer{system: func(string) error { return systemErr }, stdout: io.Discard}
	err := w.write("hello", Auto)
	if !errors.Is(err, systemErr) || !strings.Contains(err.Error(), "stdout") {
		t.Fatalf("both failure causes must be available: %v", err)
	}
}

func TestOSC52EncodedPayloadLimit(t *testing.T) {
	for _, size := range []int{56244, 56245} {
		text := strings.Repeat("x", size)
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			var out recordingOutput
			w := writer{stdout: &out, tty: true, term: "xterm"}
			err := w.write(text, OSC52)
			tooLarge := base64.StdEncoding.EncodedLen(size) > 74994
			if (err != nil) != tooLarge {
				t.Fatalf("encoded length %d: %v", base64.StdEncoding.EncodedLen(size), err)
			}
			if tooLarge && out.calls != 0 {
				t.Fatal("oversized copy must not write or truncate the clipboard")
			}
			if !tooLarge && out.String() != "\x1b]52;c;"+base64.StdEncoding.EncodeToString([]byte(text))+"\a" {
				t.Fatal("accepted payload did not arrive intact")
			}
		})
	}
}

func TestOSC52ScreenWrapping(t *testing.T) {
	var out recordingOutput
	text := strings.Repeat("x", 80)
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	w := writer{stdout: &out, tty: true, term: "screen-256color"}
	if err := w.write(text, OSC52); err != nil {
		t.Fatal(err)
	}
	want := "\x1bP\x1b]52;c;" + encoded[:76] + "\x1b\\\x1bP" + encoded[76:] + "\a\x1b\\"
	if out.String() != want || out.calls != 1 {
		t.Fatalf("screen copy must use DCS chunks in one write: calls=%d, output=%q", out.calls, out.String())
	}
}

func TestOSC52OutputFailure(t *testing.T) {
	outputErr := errors.New("terminal write failed")
	for _, tc := range []struct {
		name  string
		err   error
		short bool
		want  error
	}{
		{name: "write error", err: outputErr, want: outputErr},
		{name: "short write", short: true, want: io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := recordingOutput{err: tc.err, short: tc.short}
			w := writer{stdout: &out, tty: true, term: "xterm"}
			if err := w.write("hello", OSC52); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if out.calls != 1 {
				t.Fatalf("output calls=%d, want 1", out.calls)
			}
		})
	}
}
