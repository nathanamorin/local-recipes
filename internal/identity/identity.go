// Package identity centralizes how local-recipes resolves the current
// user's identity from the X-Auth-Request-User header that Istio's
// ext_authz gating injects once a request has passed the pocket-id-sso
// AuthorizationPolicy at the Gateway (see
// k8s-deployment/deployments/networking/pocket-id-sso.yaml). Both the
// public HTTP API (handlers) and the MCP server use this package so the
// two front doors resolve identity - and therefore which per-user MySQL
// database a request is routed to - identically.
package identity

import "net/http"

// Header is the header Istio's ext_authz gating injects with the
// authenticated Pocket ID username/subject.
const Header = "X-Auth-Request-User"

// FromRequest extracts the authenticated user ID from r. Unlike
// local-podcasts (which degrades to a shared "anonymous" user when the
// header is absent), local-recipes fails closed: every recipe/ingredient
// lives in a per-user MySQL database keyed off this value (see
// internal/dbmanager), so a request with no identity has no database to
// route to and must be rejected by the caller rather than silently
// falling back to a shared identity.
func FromRequest(r *http.Request) (string, bool) {
	v := r.Header.Get(Header)
	if v == "" {
		return "", false
	}
	return v, true
}
