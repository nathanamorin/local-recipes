// Command mcpserver runs local-recipes' MCP (Model Context Protocol)
// server, giving an LLM client (e.g. a chat assistant) tools to
// auto-import recipes and log current ingredient stock on a user's
// behalf.
//
// It's served over MCP's Streamable HTTP transport (not stdio), on its
// own port. In the default "oidc" auth mode it is its own OAuth 2.0
// protected resource with Pocket ID as the authorization server (see
// auth.go), and maps each access token's subject onto X-Auth-Request-User
// so every tool call operates on the same per-user data as the web UI.
// The "header" mode trusts X-Auth-Request-User directly and is only for
// local development.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ardanlabs/conf/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nathanamorin/local-recipes/internal/identity"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

var build = "develop"

type appConfig struct {
	DBDSN          string `conf:"env:DB_DSN,help:PostgreSQL connection string (required)"`
	Addr           string `conf:"default::8090,help:listen address for the MCP HTTP server"`
	DBMaxOpenConns int    `conf:"default:10,help:max concurrent connections in the Postgres connection pool"`

	AuthMode    string   `conf:"default:oidc,help:oidc (verify Pocket ID bearer tokens) or header (trust X-Auth-Request-User - local dev only)"`
	OIDCIssuer  string   `conf:"help:OIDC issuer URL of the authorization server (oidc mode)"`
	OIDCJWKSURL string   `conf:"help:JWKS URL; defaults to <issuer>/.well-known/jwks.json"`
	ResourceURL string   `conf:"help:this server's public MCP endpoint URL; the access token audience (oidc mode)"`
	OAuthScopes []string `conf:"default:openid;profile;offline_access,help:scopes advertised to MCP clients"`
}

// userIDContextKey is how the per-request user ID (resolved from
// identity.Header, see withUserContext) is threaded through MCP's
// context.Context into each tool handler.
type userIDContextKey struct{}

func withUserContext(ctx context.Context, r *http.Request) context.Context {
	if userID, ok := identity.FromRequest(r); ok {
		return context.WithValue(ctx, userIDContextKey{}, userID)
	}
	return ctx
}

// userIDFromContext returns the identity withUserContext attached, or an
// error if the request had no X-Auth-Request-User header - every tool
// below calls this first and fails closed rather than falling back to a
// shared/anonymous identity (see internal/identity's doc comment).
func userIDFromContext(ctx context.Context) (string, error) {
	userID, _ := ctx.Value(userIDContextKey{}).(string)
	if userID == "" {
		return "", fmt.Errorf("missing %s header - request was not authenticated", identity.Header)
	}
	return userID, nil
}

func main() {
	cfg := appConfig{}
	help, err := conf.Parse("LOCAL_RECIPES_MCP", &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			os.Exit(0)
		}
		log.Fatalf("parsing config: %s", err)
	}
	if cfg.DBDSN == "" {
		log.Fatalf("LOCAL_RECIPES_MCP_DB_DSN is required (PostgreSQL connection string)")
	}

	db, err := recipes.InitDB(cfg.DBDSN, cfg.DBMaxOpenConns)
	if err != nil {
		klog.Fatalf("error initializing database: %s", err)
	}

	mcpServer := server.NewMCPServer("local-recipes", build,
		server.WithToolCapabilities(true),
	)
	registerTools(mcpServer, db)

	mcpHandler := server.NewStreamableHTTPServer(mcpServer,
		server.WithHTTPContextFunc(withUserContext),
	)

	mux, err := newMux(cfg, mcpHandler)
	if err != nil {
		log.Fatalf("configuring auth: %s", err)
	}

	klog.Infof("starting local-recipes MCP server build[%s] addr[%s] auth[%s]", build, cfg.Addr, cfg.AuthMode)
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func newMux(cfg appConfig, mcpHandler http.Handler) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	switch cfg.AuthMode {
	case "oidc":
		auth, err := newOIDCAuth(cfg.OIDCIssuer, cfg.OIDCJWKSURL, cfg.ResourceURL, cfg.OAuthScopes)
		if err != nil {
			return nil, err
		}
		for _, p := range auth.metadataPaths() {
			mux.HandleFunc("GET "+p, auth.serveMetadata)
		}
		mux.Handle("/mcp", auth.middleware(mcpHandler))
	case "header":
		klog.Warningf("auth mode %q trusts the %s header from any caller - local development only", cfg.AuthMode, identity.Header)
		mux.Handle("/mcp", mcpHandler)
	default:
		return nil, fmt.Errorf("unknown auth mode %q (want oidc or header)", cfg.AuthMode)
	}
	return mux, nil
}

func registerTools(s *server.MCPServer, db *gorm.DB) {
	s.AddTool(importRecipeTool(), importRecipeHandler(db))
	s.AddTool(updateRecipeTool(), updateRecipeHandler(db))
	s.AddTool(logIngredientStockTool(), logIngredientStockHandler(db))
	s.AddTool(listIngredientsTool(), listIngredientsHandler(db))
	s.AddTool(listLowStockTool(), listLowStockHandler(db))
	s.AddTool(listRecipesTool(), listRecipesHandler(db))
}

// toolError wraps err as an MCP tool-level error result (rather than a
// transport-level error), so the calling LLM sees the failure reason as
// tool output it can react to/relay, instead of the call simply failing.
func toolError(err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(err.Error()), nil
}
