package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nathanamorin/local-recipes/internal/identity"
)

// userIDMiddleware reads the authenticated user ID Istio injects via
// identity.Header and stores it on the request context for handlers to
// read via currentUserID. Unlike local-podcasts (which degrades to a
// shared "anonymous" user when the header is missing), this fails
// closed: every request without an identity is rejected, since every
// query below is partitioned by user ID in the shared database and
// there's no safe anonymous bucket to fall back to.
func userIDMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID, ok := identity.FromRequest(c.Request())
		if !ok {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "missing " + identity.Header + " header",
			})
		}
		c.Set("user_id", userID)
		return next(c)
	}
}

func currentUserID(c echo.Context) string {
	v, _ := c.Get("user_id").(string)
	return v
}

type currentUserResponse struct {
	UserID string `json:"user_id"`
}

// GetCurrentUser godoc
// @Summary      Get the current authenticated user
// @Tags         user
// @Produce      json
// @Success      200  {object}  currentUserResponse
// @Router       /me [get]
func (h *Handler) GetCurrentUser(c echo.Context) error {
	return c.JSON(http.StatusOK, currentUserResponse{UserID: currentUserID(c)})
}
