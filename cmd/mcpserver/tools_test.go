package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"gorm.io/gorm"
)

func newToolTestServer(t *testing.T) *server.MCPServer {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := recipes.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("test", "test", server.WithToolCapabilities(true))
	registerTools(s, db)
	return s
}

// rpc sends one JSON-RPC message through the real MCP server, so the test
// sees exactly what a client like Claude Desktop sees.
func rpc(t *testing.T, s *server.MCPServer, method string, params any) map[string]any {
	t.Helper()
	msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	ctx := context.WithValue(context.Background(), userIDContextKey{}, "test-user")
	b, _ := json.Marshal(s.HandleMessage(ctx, msg))
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != nil {
		t.Fatalf("%s: %v", method, out["error"])
	}
	return out["result"].(map[string]any)
}

func TestToolSchemasArePublished(t *testing.T) {
	s := newToolTestServer(t)
	res := rpc(t, s, "tools/list", map[string]any{})

	wantRequired := map[string][]string{
		"import_recipe":        {"title", "body_markdown", "ingredients"},
		"log_ingredient_stock": {"ingredient"},
		"update_recipe":        {"recipe_id", "title", "body_markdown", "ingredients"},
	}
	seen := 0
	for _, raw := range res["tools"].([]any) {
		tool := raw.(map[string]any)
		name := tool["name"].(string)
		schema := tool["inputSchema"].(map[string]any)
		want, ok := wantRequired[name]
		if !ok {
			continue
		}
		seen++
		props, _ := schema["properties"].(map[string]any)
		if len(props) == 0 {
			t.Errorf("%s: published an empty input schema (struct tags not understood by the schema generator?)", name)
		}
		var got []string
		for _, r := range schema["required"].([]any) {
			got = append(got, r.(string))
		}
		for _, f := range want {
			if !slices.Contains(got, f) {
				t.Errorf("%s: %q not marked required (got %v)", name, f, got)
			}
		}
	}
	if seen != len(wantRequired) {
		t.Fatalf("only found %d of %d expected tools", seen, len(wantRequired))
	}
}

func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) map[string]any {
	t.Helper()
	return rpc(t, s, "tools/call", map[string]any{"name": name, "arguments": args})
}

func TestToolResults(t *testing.T) {
	s := newToolTestServer(t)

	t.Run("incomplete import saves nothing", func(t *testing.T) {
		res := callTool(t, s, "import_recipe", map[string]any{"title": "Pretzels"})
		if res["isError"] != true {
			t.Fatalf("expected tool error, got %v", res)
		}
		list := callTool(t, s, "list_recipes", map[string]any{})
		if got := list["structuredContent"].(map[string]any)["recipes"]; got != nil && len(got.([]any)) != 0 {
			t.Fatalf("incomplete import was saved: %v", got)
		}
	})

	res := callTool(t, s, "import_recipe", map[string]any{
		"title":         "Pretzels",
		"body_markdown": "## Steps\n1. Boil\n2. Bake",
		"ingredients":   []map[string]any{{"name": "flour", "quantity": 560, "unit": "g"}},
	})
	if res["isError"] == true {
		t.Fatalf("import failed: %v", res)
	}

	for _, name := range []string{"list_recipes", "list_ingredients", "list_low_stock_ingredients", "import_recipe"} {
		t.Run(name+" returns object structuredContent", func(t *testing.T) {
			args := map[string]any{}
			if name == "import_recipe" {
				args = map[string]any{"title": "Bagels", "body_markdown": "x", "ingredients": []map[string]any{{"name": "flour"}}}
			}
			res := callTool(t, s, name, args)
			sc := res["structuredContent"]
			if sc == nil || reflect.TypeOf(sc).Kind() != reflect.Map {
				t.Fatalf("structuredContent must be a JSON object, got %T: %v", sc, sc)
			}
			text := res["content"].([]any)[0].(map[string]any)["text"].(string)
			var fromText map[string]any
			if err := json.Unmarshal([]byte(text), &fromText); err != nil {
				t.Fatalf("text content should carry the same JSON: %q", text)
			}
		})
	}

	list := callTool(t, s, "list_recipes", map[string]any{})["structuredContent"].(map[string]any)["recipes"].([]any)
	if len(list) != 2 {
		t.Fatalf("want 2 recipes, got %d", len(list))
	}

	t.Run("update_recipe replaces a recipe", func(t *testing.T) {
		id := list[0].(map[string]any)["id"].(string)
		if res := callTool(t, s, "update_recipe", map[string]any{"recipe_id": id, "title": "only a title"}); res["isError"] != true {
			t.Fatalf("incomplete update should be rejected, got %v", res)
		}
		res := callTool(t, s, "update_recipe", map[string]any{
			"recipe_id":     id,
			"title":         "Soft Pretzels",
			"body_markdown": "## Steps\n1. Boil 30s\n2. Bake 450F",
			"servings":      8,
			"ingredients": []map[string]any{
				{"name": "bread flour", "quantity": 560, "unit": "g"},
				{"name": "baking soda", "quantity": 0.67, "unit": "cup"},
			},
		})
		if res["isError"] == true {
			t.Fatalf("update failed: %v", res)
		}
		got := res["structuredContent"].(map[string]any)
		if got["title"] != "Soft Pretzels" || got["servings"].(float64) != 8 || len(got["ingredients"].([]any)) != 2 {
			t.Fatalf("update not applied: %v", got)
		}
	})
}
