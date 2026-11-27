package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/nathanamorin/local-recipes/internal/identity"
	"k8s.io/klog/v2"
)

// MCP authorization (https://modelcontextprotocol.io/specification/latest/basic/authorization).
//
// The MCP server is its own OAuth 2.0 protected resource rather than sitting
// behind the oauth2-proxy ext_authz gate the web UI uses: that gate answers
// unauthenticated requests with a 302 to the IdP login page, which only a
// browser can follow. MCP clients instead expect a 401 whose WWW-Authenticate
// header points at this server's RFC 9728 protected-resource metadata, which
// names Pocket ID as the authorization server. The client then runs the
// OAuth authorization-code + PKCE flow against Pocket ID itself, requesting
// an access token for this server's resource URL (RFC 8707).
//
// Pocket ID issues those as RFC 9068 JWT access tokens with aud set to the
// resource URL (it must be registered as an API in Pocket ID) and sub set to
// the user's ID - the same claim oauth2-proxy forwards to the web UI as
// X-Auth-Request-User, so both front doors resolve to the same per-user data.

const protectedResourceMetadataPrefix = "/.well-known/oauth-protected-resource"

type oidcAuth struct {
	issuer      string
	resource    string
	metadataURL string
	scopes      []string
	verifier    *oidc.IDTokenVerifier
}

func newOIDCAuth(issuer, jwksURL, resource string, scopes []string) (*oidcAuth, error) {
	issuer = strings.TrimSuffix(issuer, "/")
	if issuer == "" || resource == "" {
		return nil, fmt.Errorf("oidc auth requires both an issuer and a resource URL")
	}
	u, err := url.Parse(resource)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("resource %q must be an absolute URL", resource)
	}
	if jwksURL == "" {
		jwksURL = issuer + "/.well-known/jwks.json"
	}
	// RFC 9728 section 3.1: metadata lives at the well-known prefix inserted
	// between the host and the resource's path.
	metadataURL := u.Scheme + "://" + u.Host + protectedResourceMetadataPrefix + strings.TrimSuffix(u.Path, "/")

	// NewRemoteKeySet fetches (and caches/rotates) keys lazily, so a Pocket ID
	// outage at startup doesn't crash-loop this server. ClientID here is the
	// expected audience: tokens minted for any other client or API (including
	// plain ID tokens, whose aud is the client ID) are rejected.
	keySet := oidc.NewRemoteKeySet(context.Background(), jwksURL)
	verifier := oidc.NewVerifier(issuer, keySet, &oidc.Config{ClientID: resource})

	return &oidcAuth{
		issuer:      issuer,
		resource:    resource,
		metadataURL: metadataURL,
		scopes:      scopes,
		verifier:    verifier,
	}, nil
}

// metadataPaths are the paths the protected-resource metadata is served on:
// the RFC 9728 path-suffixed location, plus the bare prefix some clients
// probe first.
func (a *oidcAuth) metadataPaths() []string {
	u, _ := url.Parse(a.metadataURL)
	paths := []string{u.Path}
	if u.Path != protectedResourceMetadataPrefix {
		paths = append(paths, protectedResourceMetadataPrefix)
	}
	return paths
}

func (a *oidcAuth) serveMetadata(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"resource":                 a.resource,
		"authorization_servers":    []string{a.issuer},
		"scopes_supported":         a.scopes,
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "local-recipes",
	})
}

// middleware authenticates the bearer token and, on success, sets
// identity.Header from the token's subject so the rest of the server keeps
// resolving identity exactly as before. Any client-supplied identity header
// is discarded first: with the ext_authz gate gone it would otherwise let a
// caller pick any user.
func (a *oidcAuth) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(identity.Header)

		raw, ok := bearerToken(r)
		if !ok {
			a.challenge(w, "")
			return
		}
		tok, err := a.verifier.Verify(r.Context(), raw)
		if err != nil {
			klog.V(1).Infof("rejecting MCP request: %s", err)
			a.challenge(w, "invalid_token")
			return
		}
		if tok.Subject == "" {
			a.challenge(w, "invalid_token")
			return
		}
		r.Header.Set(identity.Header, tok.Subject)
		next.ServeHTTP(w, r)
	})
}

func (a *oidcAuth) challenge(w http.ResponseWriter, errCode string) {
	v := fmt.Sprintf(`Bearer resource_metadata=%q`, a.metadataURL)
	if errCode != "" {
		v += fmt.Sprintf(`, error=%q`, errCode)
	}
	if len(a.scopes) > 0 {
		v += fmt.Sprintf(`, scope=%q`, strings.Join(a.scopes, " "))
	}
	w.Header().Set("WWW-Authenticate", v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	scheme, tok, found := strings.Cut(h, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	tok = strings.TrimSpace(tok)
	return tok, tok != ""
}
