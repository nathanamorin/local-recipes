package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nathanamorin/local-recipes/internal/identity"
)

// With the forwarded access token expired, userinfo fails - /api/me must
// still return the Pocket ID picture built from the user ID.
func TestGetCurrentUserPictureFallback(t *testing.T) {
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer idp.Close()

	e := echo.New()
	New(nil,
		WithProfiles(identity.NewProfileFetcher(idp.URL)),
		WithPictureURLTemplate("https://id.example.test/api/users/{sub}/profile-picture.png"),
	).Register(e)

	req := httptest.NewRequest(http.MethodGet, APIPrefix+"/me", nil)
	req.Header.Set(identity.Header, "user-123")
	req.Header.Set(identity.EmailHeader, "me@example.test")
	req.Header.Set(identity.AccessTokenHeader, "expired")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if got["picture"] != "https://id.example.test/api/users/user-123/profile-picture.png" || got["email"] != "me@example.test" {
		t.Fatalf("unexpected profile: %v", got)
	}
}
