package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	sessionCookie = "b9s_session"
	csrfHeader    = "X-B9s-CSRF"
	secretBytes   = 32
)

// Auth derives the pairing token, the session cookie and the CSRF value from
// one secret. Keeping the secret in a file means a paired phone stays paired
// across restarts, and replacing the file unpairs every device at once.
type Auth struct {
	secret []byte
}

// NewAuth returns an Auth for secret. A nil Auth accepts every request; only
// a loopback server may run without one (see CheckListen).
func NewAuth(secret []byte) (*Auth, error) {
	if len(secret) < 16 {
		return nil, errors.New("web auth secret is shorter than 16 bytes")
	}
	return &Auth{secret: append([]byte(nil), secret...)}, nil
}

// LoadOrCreateSecret reads the secret at path, or writes a new random one
// readable only by the owner. renew replaces an existing secret.
func LoadOrCreateSecret(path string, renew bool) ([]byte, error) {
	if !renew {
		data, err := os.ReadFile(path)
		if err == nil {
			secret, decErr := hex.DecodeString(strings.TrimSpace(string(data)))
			if decErr == nil && len(secret) >= 16 {
				return secret, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	secret := make([]byte, secretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(secret)+"\n"), 0o600); err != nil {
		return nil, err
	}
	return secret, nil
}

func (a *Auth) derive(purpose string) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(purpose))
	return hex.EncodeToString(mac.Sum(nil))
}

// PairToken is the secret part of the pairing URL.
func (a *Auth) PairToken() string { return a.derive("pair-v1")[:32] }

func (a *Auth) sessionValue() string { return a.derive("session-v1") }

// CSRF is the value every write must carry in the X-B9s-CSRF header. A page
// on another site cannot set that header without a CORS preflight, which this
// server never grants.
func (a *Auth) CSRF() string { return a.derive("csrf-v1")[:32] }

func equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// paired reports whether r carries the session cookie.
func (a *Auth) paired(r *http.Request) bool {
	if a == nil {
		return true
	}
	c, err := r.Cookie(sessionCookie)
	return err == nil && equal(c.Value, a.sessionValue())
}

// csrfOK reports whether a write carries the CSRF header.
func (a *Auth) csrfOK(r *http.Request) bool {
	if a == nil {
		return true
	}
	return equal(r.Header.Get(csrfHeader), a.CSRF())
}

// pair trades a valid token for the session cookie and sends the browser to
// the app.
func (a *Auth) pair(w http.ResponseWriter, r *http.Request) {
	if a == nil {
		redirectToApp(w)
		return
	}
	if !equal(r.URL.Query().Get("t"), a.PairToken()) {
		http.Error(w, "This pairing link is not valid. Copy the link b9s web printed again.", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    a.sessionValue(),
		Path:     "/",
		MaxAge:   90 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	redirectToApp(w)
}

// redirectToApp sends the browser to the app next to /pair. http.Redirect
// would make "./" absolute against the path this server sees, which drops a
// prefix such as the one Tailscale Serve strips.
func redirectToApp(w http.ResponseWriter) {
	w.Header().Set("Location", "./")
	w.WriteHeader(http.StatusSeeOther)
}

// CheckListen refuses an address other than loopback when the server has no
// auth. Every tailnet or LAN peer could otherwise read and write issues.
func CheckListen(addr string, auth *Auth) error {
	if auth != nil {
		return nil
	}
	if !IsLoopbackAddr(addr) {
		return fmt.Errorf("refusing to listen on %s without a pairing token: only a loopback address may skip it", addr)
	}
	return nil
}

// IsLoopbackAddr reports whether a listen address binds only to loopback. An
// empty host binds every interface, so it is not loopback.
func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
