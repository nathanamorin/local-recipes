package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ardanlabs/conf/v3"
	"github.com/labstack/echo/v4"
	"github.com/nathanamorin/local-recipes/handlers"
	"github.com/nathanamorin/local-recipes/internal/identity"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"k8s.io/klog/v2"
)

var build = "develop"

type appConfig struct {
	// Explicit env name: conf's default camelCase->SCREAMING_SNAKE_CASE
	// derivation doesn't split consecutive capitals, so "DBDSN" would
	// otherwise become LOCAL_RECIPES_DBDSN rather than the documented/
	// expected LOCAL_RECIPES_DB_DSN.
	DBDSN          string `conf:"env:DB_DSN,help:PostgreSQL connection string, e.g. postgres://user:password@host:5432/dbname?sslmode=disable (required)"`
	Addr           string `conf:"default::8080,help:listen address for the public HTTP server"`
	DBMaxOpenConns int    `conf:"default:20,help:max concurrent connections in the Postgres connection pool"`
	StaticDir      string `conf:"default:static,help:directory to serve the built frontend from"`
	UserInfoURL    string `conf:"help:OIDC userinfo endpoint used to show the signed-in user's name and picture (optional)"`
}

// @title           local-recipes API
// @version         1.0
// @description     API for managing recipes, ingredient stock, ratings, and meal scheduling.
// @license.name    MIT

// @BasePath  /
func main() {
	cfg := appConfig{}
	help, err := conf.Parse("LOCAL_RECIPES", &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			os.Exit(0)
		}
		log.Fatalf("parsing config: %s", err)
	}

	if cfg.DBDSN == "" {
		log.Fatalf("LOCAL_RECIPES_DB_DSN is required (PostgreSQL connection string)")
	}

	klog.Infof("starting local-recipes build[%s] addr[%s]", build, cfg.Addr)

	db, err := recipes.InitDB(cfg.DBDSN, cfg.DBMaxOpenConns)
	if err != nil {
		klog.Fatalf("error initializing database: %s", err)
	}

	e := echo.New()

	var opts []handlers.Option
	if cfg.UserInfoURL != "" {
		opts = append(opts, handlers.WithProfiles(identity.NewProfileFetcher(cfg.UserInfoURL)))
	}
	h := handlers.New(db, opts...)
	h.Register(e)

	// Unauthenticated liveness/readiness target for the k8s Deployment's
	// probes - /me (and everything else under handlers.Register) requires
	// X-Auth-Request-User and would otherwise always 401 without a
	// logged-in identity, permanently failing readiness.
	e.GET("/healthz", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// Serve static files and fall back to index.html for SPA client-side routes.
	e.GET("/*", func(c echo.Context) error {
		p := filepath.Join(cfg.StaticDir, c.Param("*"))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return c.File(p)
		}
		return c.File(filepath.Join(cfg.StaticDir, "index.html"))
	})

	if err := e.Start(cfg.Addr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
