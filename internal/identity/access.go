package identity

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Nich728/fuxi/internal/platform/clock"
)

// ErrUnauthenticated means the request carries no valid sign-in.
var ErrUnauthenticated = errors.New("not signed in")

// Authenticator turns a request into the signed-in user's email.
type Authenticator interface {
	Authenticate(r *http.Request) (email string, err error)
}

// AccessHeader is where Cloudflare Access puts its signed token on every
// request it forwards to the origin.
const AccessHeader = "Cf-Access-Jwt-Assertion"

// accessCookie is the same token as a cookie, set on the app's own domain.
const accessCookie = "CF_Authorization"

// Access verifies Cloudflare Access tokens (ADR-0018). Every request is
// checked here, so the API stays closed even if someone reaches the origin
// without going through Cloudflare.
//
// Tokens are RS256 JWTs signed by the team's keys, published at
// https://<team>.cloudflareaccess.com/cdn-cgi/access/certs. The audience is
// the Access application's AUD tag.
type Access struct {
	issuer   string
	audience string
	certsURL string
	client   *http.Client
	clock    clock.Clock

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// leeway tolerates small clock differences between Cloudflare and the server.
const leeway = time.Minute

// refetchEvery bounds how often an unknown key id triggers a new key fetch.
const refetchEvery = time.Minute

// NewAccess returns a verifier for the Access team domain (for example
// "https://fuxi.cloudflareaccess.com") and application audience tag.
func NewAccess(teamDomain, audience string, clk clock.Clock, client *http.Client) (*Access, error) {
	teamDomain = strings.TrimRight(teamDomain, "/")
	if !strings.HasPrefix(teamDomain, "https://") {
		return nil, fmt.Errorf("access team domain %q must start with https://", teamDomain)
	}
	if audience == "" {
		return nil, errors.New("access audience is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Access{
		issuer:   teamDomain,
		audience: audience,
		certsURL: teamDomain + "/cdn-cgi/access/certs",
		client:   client,
		clock:    clk,
	}, nil
}

// Authenticate reads the token from the header (or, failing that, the cookie)
// and returns the verified email.
func (a *Access) Authenticate(r *http.Request) (string, error) {
	token := r.Header.Get(AccessHeader)
	if token == "" {
		if c, err := r.Cookie(accessCookie); err == nil {
			token = c.Value
		}
	}
	if token == "" {
		return "", ErrUnauthenticated
	}
	claims, err := a.Verify(r.Context(), token)
	if err != nil {
		return "", err
	}
	return claims.Email, nil
}

// Claims are the fields Fuxi uses from an Access token.
type Claims struct {
	Email     string   `json:"email"`
	Subject   string   `json:"sub"`
	Issuer    string   `json:"iss"`
	Audience  audience `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	NotBefore int64    `json:"nbf"`
	IssuedAt  int64    `json:"iat"`
}

// audience accepts both forms the JWT spec allows: a string or a list.
type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*a = audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

// Verify checks the token's signature, issuer, audience and lifetime.
// Every failure wraps ErrUnauthenticated.
func (a *Access) Verify(ctx context.Context, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("%w: malformed token", ErrUnauthenticated)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return Claims{}, fmt.Errorf("%w: header: %v", ErrUnauthenticated, err)
	}
	// Only RS256: never let the token choose a weaker or "none" algorithm.
	if header.Alg != "RS256" {
		return Claims{}, fmt.Errorf("%w: unexpected alg %q", ErrUnauthenticated, header.Alg)
	}
	key, err := a.key(ctx, header.Kid)
	if err != nil {
		return Claims{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: signature encoding", ErrUnauthenticated)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return Claims{}, fmt.Errorf("%w: bad signature", ErrUnauthenticated)
	}

	var c Claims
	if err := decodeSegment(parts[1], &c); err != nil {
		return Claims{}, fmt.Errorf("%w: claims: %v", ErrUnauthenticated, err)
	}
	now := a.clock.Now()
	switch {
	case c.Issuer != a.issuer:
		return Claims{}, fmt.Errorf("%w: issuer %q", ErrUnauthenticated, c.Issuer)
	case !slices.Contains(c.Audience, a.audience):
		return Claims{}, fmt.Errorf("%w: wrong audience", ErrUnauthenticated)
	case c.ExpiresAt == 0 || now.After(time.Unix(c.ExpiresAt, 0).Add(leeway)):
		return Claims{}, fmt.Errorf("%w: token expired", ErrUnauthenticated)
	case c.NotBefore != 0 && now.Add(leeway).Before(time.Unix(c.NotBefore, 0)):
		return Claims{}, fmt.Errorf("%w: token not valid yet", ErrUnauthenticated)
	case c.Email == "":
		// Service tokens carry no email; Fuxi only serves people.
		return Claims{}, fmt.Errorf("%w: token has no email", ErrUnauthenticated)
	}
	return c, nil
}

// key returns the public key for kid, fetching the team's keys when kid is
// unknown (Cloudflare rotates them every few weeks).
func (a *Access) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if k, ok := a.keys[kid]; ok {
		return k, nil
	}
	if !a.fetchedAt.IsZero() && a.clock.Now().Sub(a.fetchedAt) < refetchEvery {
		return nil, fmt.Errorf("%w: unknown key id", ErrUnauthenticated)
	}
	keys, err := a.fetchKeys(ctx)
	a.fetchedAt = a.clock.Now()
	if err != nil {
		// A key-server outage is ours, not the user's: report it as such.
		return nil, fmt.Errorf("fetch access keys: %w", err)
	}
	a.keys = keys
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("%w: unknown key id", ErrUnauthenticated)
}

func (a *Access) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.certsURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d", a.certsURL, resp.StatusCode)
	}
	var set struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("decode keys: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil || len(e) > 4 {
			return nil, fmt.Errorf("key %q: bad encoding", k.Kid)
		}
		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("no RSA keys in key set")
	}
	return keys, nil
}

func decodeSegment(seg string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Dev signs every request in as a fixed email, or as the email in the
// X-Fuxi-Dev-Email header. It exists for local development and tests and
// must never run in production (FUXI_AUTH=dev).
type Dev struct {
	DefaultEmail string
}

// DevHeader overrides the dev user's email per request.
const DevHeader = "X-Fuxi-Dev-Email"

// Authenticate returns the header's email or the default.
func (d Dev) Authenticate(r *http.Request) (string, error) {
	if e := strings.TrimSpace(r.Header.Get(DevHeader)); e != "" {
		return e, nil
	}
	if d.DefaultEmail == "" {
		return "", ErrUnauthenticated
	}
	return d.DefaultEmail, nil
}
