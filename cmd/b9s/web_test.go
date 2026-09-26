package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWebRefusesNoTokenOffLoopback(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	for _, addr := range []string{"0.0.0.0:0", ":0", "100.64.0.1:0"} {
		var out, errOut bytes.Buffer
		code := runWeb([]string{"--no-token", "--listen", addr}, &out, &errOut)
		if code != 2 || !strings.Contains(errOut.String(), "without a pairing token") {
			t.Errorf("%s: exit %d, stderr %q", addr, code, errOut.String())
		}
	}
}

func TestWebTrustHeaderNeedsOwnerAndAToken(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	for _, args := range [][]string{
		{"--trust-header", "X-Forwarded-Email"},
		{"--owner", "a@b.c"},
		{"--trust-header", "X-Forwarded-Email", "--owner", "a@b.c", "--no-token"},
	} {
		var out, errOut bytes.Buffer
		if code := runWeb(args, &out, &errOut); code != 2 {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut.String())
		}
	}
}

func TestWebHelpExitsCleanly(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWeb([]string{"--help"}, &out, &errOut); code != 0 || !strings.Contains(errOut.String(), "pairing link") {
		t.Fatalf("exit %d, usage %q", code, errOut.String())
	}
}

func TestPairURLCarriesTokenOnlyWithAuth(t *testing.T) {
	if got := pairURL("http://localhost:7979", nil); got != "http://localhost:7979/" {
		t.Fatalf("no auth: %q", got)
	}
}
