package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/nathanamorin/local-recipes/internal/identity"
)

const testResource = "https://recipes-mcp.example.test/mcp"

type fakeIssuer struct {
	srv    *httptest.Server
	key    *rsa.PrivateKey
	signer jose.Signer
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("at+jwt").WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	return &fakeIssuer{srv: srv, key: key, signer: signer}
}

func (f *fakeIssuer) token(t *testing.T, sub string, aud []string, exp time.Time) string {
	t.Helper()
	s, err := jwt.Signed(f.signer).Claims(jwt.Claims{
		Issuer:   f.srv.URL,
		Subject:  sub,
		Audience: aud,
		Expiry:   jwt.NewNumericDate(exp),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newTestMux(t *testing.T, f *fakeIssuer) http.Handler {
	t.Helper()
	echoUser := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get(identity.Header)))
	})
	mux, err := newMux(appConfig{
		AuthMode:    "oidc",
		OIDCIssuer:  f.srv.URL,
		OIDCJWKSURL: f.srv.URL + "/jwks",
		ResourceURL: testResource,
		OAuthScopes: []string{"openid", "offline_access"},
	}, echoUser)
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

func do(h http.Handler, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMCPAuth(t *testing.T) {
	f := newFakeIssuer(t)
	h := newTestMux(t, f)
	wantMeta := `resource_metadata="https://recipes-mcp.example.test/.well-known/oauth-protected-resource/mcp"`

	t.Run("no token is challenged with resource metadata", func(t *testing.T) {
		rec := do(h, http.MethodPost, "/mcp", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, wantMeta) {
			t.Fatalf("WWW-Authenticate %q missing %s", got, wantMeta)
		}
	})

	t.Run("spoofed identity header without token is rejected", func(t *testing.T) {
		rec := do(h, http.MethodPost, "/mcp", map[string]string{identity.Header: "someone-else"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
	})

	t.Run("valid token maps subject to identity header", func(t *testing.T) {
		tok := f.token(t, "user-123", []string{testResource}, time.Now().Add(time.Hour))
		rec := do(h, http.MethodPost, "/mcp", map[string]string{
			"Authorization": "Bearer " + tok,
			identity.Header: "someone-else",
		})
		if rec.Code != http.StatusOK || rec.Body.String() != "user-123" {
			t.Fatalf("got %d %q, want 200 user-123", rec.Code, rec.Body.String())
		}
	})

	for name, tok := range map[string]string{
		"wrong audience": f.token(t, "user-123", []string{"some-other-client"}, time.Now().Add(time.Hour)),
		"expired":        f.token(t, "user-123", []string{testResource}, time.Now().Add(-time.Minute)),
		"garbage":        "not-a-jwt",
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			rec := do(h, http.MethodPost, "/mcp", map[string]string{"Authorization": "Bearer " + tok})
			if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Header().Get("WWW-Authenticate"), `error="invalid_token"`) {
				t.Fatalf("got %d %q, want 401 invalid_token", rec.Code, rec.Header().Get("WWW-Authenticate"))
			}
		})
	}

	t.Run("protected resource metadata", func(t *testing.T) {
		for _, p := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource"} {
			rec := do(h, http.MethodGet, p, nil)
			var md struct {
				Resource             string   `json:"resource"`
				AuthorizationServers []string `json:"authorization_servers"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &md); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("GET %s: %d %s", p, rec.Code, rec.Body.String())
			}
			if md.Resource != testResource || len(md.AuthorizationServers) != 1 || md.AuthorizationServers[0] != f.srv.URL {
				t.Fatalf("GET %s: unexpected metadata %+v", p, md)
			}
		}
	})

	t.Run("healthz is unauthenticated", func(t *testing.T) {
		if rec := do(h, http.MethodGet, "/healthz", nil); rec.Code != http.StatusOK {
			t.Fatalf("got %d", rec.Code)
		}
	})
}
