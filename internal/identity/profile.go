package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Headers oauth2-proxy sets alongside Header once a request passes the
// gate (see headersToUpstreamOnAllow on the pocket-id-oauth2-proxy-*
// extension providers in k8s-deployment's istiod config).
const (
	EmailHeader       = "X-Auth-Request-Email"
	AccessTokenHeader = "X-Auth-Request-Access-Token"
)

// Profile is the display information shown for the signed-in user.
type Profile struct {
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Picture  string `json:"picture,omitempty"`
}

// ProfileFetcher looks up a user's profile from the OIDC provider's
// userinfo endpoint using the access token oauth2-proxy forwards, caching
// results per user so the IdP isn't hit on every page load.
type ProfileFetcher struct {
	userInfoURL string
	ttl         time.Duration
	client      *http.Client

	mu    sync.Mutex
	cache map[string]cachedProfile
}

type cachedProfile struct {
	profile Profile
	expires time.Time
}

func NewProfileFetcher(userInfoURL string) *ProfileFetcher {
	return &ProfileFetcher{
		userInfoURL: userInfoURL,
		ttl:         10 * time.Minute,
		client:      &http.Client{Timeout: 5 * time.Second},
		cache:       map[string]cachedProfile{},
	}
}

// Fetch returns userID's profile. The userinfo response's sub must equal
// userID, so a token belonging to a different user can never populate
// (or poison the cache for) this identity.
func (f *ProfileFetcher) Fetch(ctx context.Context, userID, accessToken string) (Profile, error) {
	f.mu.Lock()
	if c, ok := f.cache[userID]; ok && time.Now().Before(c.expires) {
		f.mu.Unlock()
		return c.profile, nil
	}
	f.mu.Unlock()

	if accessToken == "" {
		return Profile{}, fmt.Errorf("no access token forwarded")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.userInfoURL, nil)
	if err != nil {
		return Profile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := f.client.Do(req)
	if err != nil {
		return Profile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Profile{}, fmt.Errorf("userinfo returned %d", resp.StatusCode)
	}

	var claims struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		Name              string `json:"name"`
		DisplayName       string `json:"display_name"`
		PreferredUsername string `json:"preferred_username"`
		Picture           string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return Profile{}, fmt.Errorf("decoding userinfo: %w", err)
	}
	if claims.Sub != userID {
		return Profile{}, fmt.Errorf("userinfo subject does not match the authenticated user")
	}

	p := Profile{
		Email:    claims.Email,
		Name:     firstNonEmpty(claims.DisplayName, claims.Name),
		Username: claims.PreferredUsername,
		Picture:  claims.Picture,
	}
	f.mu.Lock()
	f.cache[userID] = cachedProfile{profile: p, expires: time.Now().Add(f.ttl)}
	f.mu.Unlock()
	return p, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
