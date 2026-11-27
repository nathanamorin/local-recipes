// Command mcpserver runs local-recipes' MCP (Model Context Protocol)
// server, giving an LLM client (e.g. a chat assistant) tools to
// auto-import recipes and log current ingredient stock on a user's
// behalf.
//
// It's served over MCP's Streamable HTTP transport (not stdio), on its
// own port, so it can sit behind the same Istio ext_authz Pocket ID
// gating as the main app (see k8s-deployment/deployments/networking/
// pocket-id-sso.yaml) and resolve X-Auth-Request-User exactly like
// handlers/user.go does - every tool call operates on the same per-user
// data as the web UI, scoped by the same identity.
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
		return "", fmt.Errorf("missing %s header - this MCP server must be accessed through the identity-gated host", identity.Header)
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

	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithHTTPContextFunc(withUserContext),
	)

	klog.Infof("starting local-recipes MCP server build[%s] addr[%s]", build, cfg.Addr)
	if err := httpServer.Start(cfg.Addr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func registerTools(s *server.MCPServer, db *gorm.DB) {
	s.AddTool(importRecipeTool(), importRecipeHandler(db))
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
