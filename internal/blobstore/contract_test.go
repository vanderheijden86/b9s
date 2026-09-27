package blobstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// errStopList is the sentinel a List callback returns to check that List
// stops walking on the first error instead of collecting every blob first.
var errStopList = errors.New("stop listing")

// prefixed wraps s so every key gets segment prepended, giving a contract
// run against a shared, persistent bucket its own disposable namespace
// without ever emptying the bucket between runs.
type prefixedStore struct {
	Store
	segment string
}

func prefixed(s Store, segment string) Store {
	return &prefixedStore{Store: s, segment: segment}
}

func (p *prefixedStore) full(key string) string {
	return p.segment + "/" + key
}

func (p *prefixedStore) Put(ctx context.Context, key string, r io.Reader, size int64, mimeType string) error {
	return p.Store.Put(ctx, p.full(key), r, size, mimeType)
}

func (p *prefixedStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return p.Store.Open(ctx, p.full(key))
}

func (p *prefixedStore) Stat(ctx context.Context, key string) (Info, error) {
	info, err := p.Store.Stat(ctx, p.full(key))
	if err != nil {
		return Info{}, err
	}
	info.Key = strings.TrimPrefix(info.Key, p.segment+"/")
	return info, nil
}

func (p *prefixedStore) Delete(ctx context.Context, key string) error {
	return p.Store.Delete(ctx, p.full(key))
}

func (p *prefixedStore) List(ctx context.Context, prefix string, fn func(Info) error) error {
	return p.Store.List(ctx, p.full(prefix), func(i Info) error {
		i.Key = strings.TrimPrefix(i.Key, p.segment+"/")
		return fn(i)
	})
}

func (p *prefixedStore) URL(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	return p.Store.URL(ctx, p.full(key), ttl, filename)
}

// runContract checks the behaviour bd attachment relies on. Each backend's test
// calls it with a fresh, empty store.
func runContract(t *testing.T, s Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	key := "ws/db/sha256/ab/abcdef0123"
	body := []byte("hello attachment")

	if _, err := s.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat on empty store: err = %v, want ErrNotFound", err)
	}
	for i := 0; i < 2; i++ { // second Put is the idempotent retry
		if err := s.Put(ctx, key, bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
			t.Fatalf("Put #%d: %v", i+1, err)
		}
	}
	info, err := s.Stat(ctx, key)
	if err != nil || info.Size != int64(len(body)) {
		t.Fatalf("Stat = %+v, %v", info, err)
	}
	rc, err := s.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, body) {
		t.Fatalf("Open read %q", got)
	}
	var listed []string
	if err := s.List(ctx, "ws/db/", func(i Info) error { listed = append(listed, i.Key); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0] != key {
		t.Fatalf("List = %v", listed)
	}

	secondKey := "ws/db/sha256/cd/abcdef0456"
	if err := s.Put(ctx, secondKey, bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatalf("Put second blob: %v", err)
	}
	calls := 0
	err = s.List(ctx, "ws/db/", func(Info) error {
		calls++
		return errStopList
	})
	if !errors.Is(err, errStopList) {
		t.Fatalf("List with a failing callback: err = %v, want errStopList", err)
	}
	if calls != 1 {
		t.Fatalf("List called fn %d times after it returned an error, want 1", calls)
	}
	if err := s.Delete(ctx, secondKey); err != nil {
		t.Fatal(err)
	}

	if u, err := s.URL(ctx, key, time.Minute, "hello.txt"); err != nil || u == "" {
		t.Fatalf("URL = %q, %v", u, err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open after Delete: err = %v, want ErrNotFound", err)
	}
}
