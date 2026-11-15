package identity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nich728/fuxi/internal/platform/clock"
)

const testAud = "aud-tag-123"

// fakeAccess is a stand-in for a Cloudflare Access team: it serves a key
// set and signs tokens.
type fakeAccess struct {
	srv     *httptest.Server
	keys    map[string]*rsa.PrivateKey
	fetches atomic.Int32
}

func newFakeAccess(t *testing.T, kids ...string) *fakeAccess {
	t.Helper()
	f := &fakeAccess{keys: map[string]*rsa.PrivateKey{}}
	for _, kid := range kids {
		f.addKey(t, kid)
	}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdn-cgi/access/certs" {
			http.NotFound(w, r)
			return
		}
		f.fetches.Add(1)
		type jwk struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		}
		var set struct {
			Keys []jwk `json:"keys"`
		}
		for kid, k := range f.keys {
			set.Keys = append(set.Keys, jwk{
				Kid: kid, Kty: "RSA", Alg: "RS256",
				N: b64(k.N.Bytes()), E: b64(big.NewInt(int64(k.E)).Bytes()),
			})
		}
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAccess) addKey(t *testing.T, kid string) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f.keys[kid] = k
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (f *fakeAccess) token(t *testing.T, kid, alg string, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": alg, "kid": kid, "typ": "JWT"})
	c, _ := json.Marshal(claims)
	signing := b64(h) + "." + b64(c)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.keys[kid], crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + b64(sig)
}

func TestAccessVerify(t *testing.T) {
	f := newFakeAccess(t, "k1")
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	v, err := NewAccess(f.srv.URL, testAud, clk, f.srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	good := func() map[string]any {
		return map[string]any{
			"email": "andi@example.com", "sub": "u1", "iss": f.srv.URL, "aud": []string{testAud},
			"iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		}
	}
	with := func(k string, val any) map[string]any {
		c := good()
		if val == nil {
			delete(c, k)
		} else {
			c[k] = val
		}
		return c
	}

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"valid", f.token(t, "k1", "RS256", good()), false},
		{"audience as a string", f.token(t, "k1", "RS256", with("aud", testAud)), false},
		{"expired", f.token(t, "k1", "RS256", with("exp", now.Add(-2*time.Minute).Unix())), true},
		{"expired within leeway", f.token(t, "k1", "RS256", with("exp", now.Add(-30*time.Second).Unix())), false},
		{"no expiry", f.token(t, "k1", "RS256", with("exp", nil)), true},
		{"not yet valid", f.token(t, "k1", "RS256", with("nbf", now.Add(5*time.Minute).Unix())), true},
		{"wrong audience", f.token(t, "k1", "RS256", with("aud", []string{"other-app"})), true},
		{"wrong issuer", f.token(t, "k1", "RS256", with("iss", "https://evil.cloudflareaccess.com")), true},
		{"service token without email", f.token(t, "k1", "RS256", with("email", nil)), true},
		{"alg none", noneToken(good()), true},
		{"alg HS256", f.token(t, "k1", "HS256", good()), true},
		{"tampered claims", tamper(f.token(t, "k1", "RS256", good())), true},
		{"garbage", "not.a.jwt", true},
		{"two segments", "abc.def", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := v.Verify(context.Background(), tt.token)
			if tt.wantErr {
				if !errors.Is(err, ErrUnauthenticated) {
					t.Fatalf("err = %v, want ErrUnauthenticated", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.Email != "andi@example.com" {
				t.Errorf("email = %q", c.Email)
			}
		})
	}
}

func noneToken(claims map[string]any) string {
	h, _ := json.Marshal(map[string]string{"alg": "none", "kid": "k1"})
	c, _ := json.Marshal(claims)
	return b64(h) + "." + b64(c) + "."
}

// tamper swaps the claims for others with a different email, keeping the signature.
func tamper(tok string) string {
	parts := strings.Split(tok, ".")
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var c map[string]any
	_ = json.Unmarshal(raw, &c)
	c["email"] = "owner@example.com"
	b, _ := json.Marshal(c)
	return parts[0] + "." + b64(b) + "." + parts[2]
}

func TestAccessKeyRotation(t *testing.T) {
	f := newFakeAccess(t, "k1")
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	v, _ := NewAccess(f.srv.URL, testAud, clk, f.srv.Client())
	claims := map[string]any{"email": "a@example.com", "iss": f.srv.URL, "aud": testAud, "exp": now.Add(time.Hour).Unix()}

	if _, err := v.Verify(context.Background(), f.token(t, "k1", "RS256", claims)); err != nil {
		t.Fatal(err)
	}
	// Cloudflare adds a new key. A token signed with it triggers one refetch,
	// but not more than once a minute.
	f.addKey(t, "k2")
	if _, err := v.Verify(context.Background(), f.token(t, "k2", "RS256", claims)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("within a minute of the last fetch: err = %v", err)
	}
	clk.Advance(2 * time.Minute)
	claims["exp"] = clk.Now().Add(time.Hour).Unix()
	if _, err := v.Verify(context.Background(), f.token(t, "k2", "RS256", claims)); err != nil {
		t.Fatalf("after refetch: %v", err)
	}
	if got := f.fetches.Load(); got != 2 {
		t.Errorf("key fetches = %d, want 2", got)
	}
	// Known keys don't refetch.
	if _, err := v.Verify(context.Background(), f.token(t, "k1", "RS256", claims)); err != nil {
		t.Fatal(err)
	}
	if got := f.fetches.Load(); got != 2 {
		t.Errorf("key fetches = %d, want 2", got)
	}
}

func TestAccessAuthenticate(t *testing.T) {
	f := newFakeAccess(t, "k1")
	now := time.Now().UTC()
	v, _ := NewAccess(f.srv.URL+"/", testAud, clock.NewFake(now), f.srv.Client())
	tok := f.token(t, "k1", "RS256", map[string]any{
		"email": "a@example.com", "iss": f.srv.URL, "aud": testAud, "exp": now.Add(time.Hour).Unix(),
	})

	header := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	header.Header.Set(AccessHeader, tok)
	cookie := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	cookie.AddCookie(&http.Cookie{Name: "CF_Authorization", Value: tok})
	for name, r := range map[string]*http.Request{"header": header, "cookie": cookie} {
		if email, err := v.Authenticate(r); err != nil || email != "a@example.com" {
			t.Errorf("%s: email=%q err=%v", name, email, err)
		}
	}
	if _, err := v.Authenticate(httptest.NewRequest(http.MethodGet, "/v1/me", nil)); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("no token: err = %v", err)
	}
}

func TestNewAccessRejectsConfig(t *testing.T) {
	if _, err := NewAccess("http://team.cloudflareaccess.com", testAud, clock.System{}, nil); err == nil {
		t.Error("plain http accepted")
	}
	if _, err := NewAccess("https://team.cloudflareaccess.com", "", clock.System{}, nil); err == nil {
		t.Error("empty audience accepted")
	}
}

func TestDev(t *testing.T) {
	d := Dev{DefaultEmail: "dev@fuxi.localhost"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if e, _ := d.Authenticate(r); e != "dev@fuxi.localhost" {
		t.Errorf("default = %q", e)
	}
	r.Header.Set(DevHeader, "b@example.com")
	if e, _ := d.Authenticate(r); e != "b@example.com" {
		t.Errorf("header = %q", e)
	}
	if _, err := (Dev{}).Authenticate(httptest.NewRequest(http.MethodGet, "/", nil)); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("no default: err = %v", err)
	}
}
