package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// TestRoutesDoNotShadowSPA guards against API group routes (and the
// catch-alls Echo's Group.Use registers) swallowing the SPA's root and
// client-side routes, which main.go serves via a GET "/*" fallback.
func TestRoutesDoNotShadowSPA(t *testing.T) {
	e := echo.New()
	New(nil).Register(e)
	e.GET("/*", func(c echo.Context) error { return c.String(http.StatusOK, "spa") })

	for _, path := range []string{"/", "/ingredients", "/schedule", "/recipes/abc", "/recipes/new"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "spa" {
			t.Errorf("GET %s: got %d %q, want SPA fallback", path, rec.Code, rec.Body.String())
		}
	}

	for _, path := range []string{APIPrefix + "/me", APIPrefix + "/recipes", APIPrefix + "/nope"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without identity: got %d, want 401", path, rec.Code)
		}
	}
}
