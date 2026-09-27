package blobstore

import "testing"

func TestKeyIsContentAddressedAndScoped(t *testing.T) {
	const hash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	got, err := Key("osenco", "b9s", "sha256", hash)
	if err != nil {
		t.Fatal(err)
	}
	want := "osenco/b9s/sha256/9f/" + hash
	if got != want {
		t.Fatalf("Key() = %q, want %q", got, want)
	}
}

func TestKeyWithoutPrefix(t *testing.T) {
	const hash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	got, err := Key("", "b9s", "sha256", hash)
	if err != nil {
		t.Fatal(err)
	}
	if got != "b9s/sha256/9f/"+hash {
		t.Fatalf("Key() = %q", got)
	}
}

func TestKeyRejectsUnsafeParts(t *testing.T) {
	cases := []struct{ prefix, db, algo, hash string }{
		{"../x", "b9s", "sha256", "9f86d081"},
		{"", "b9s/../../etc", "sha256", "9f86d081"},
		{"", "b9s", "md5", "9f86d081"},
		{"", "b9s", "sha256", "NOT-HEX"},
		{"", "b9s", "sha256", "9f"},
		{"", "", "sha256", "9f86d081"},
	}
	for _, c := range cases {
		if _, err := Key(c.prefix, c.db, c.algo, c.hash); err == nil {
			t.Errorf("Key(%q,%q,%q,%q) accepted unsafe input", c.prefix, c.db, c.algo, c.hash)
		}
	}
}
