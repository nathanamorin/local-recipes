# local-recipes

A self-hosted recipe manager: a Markdown-bodied recipe library, ingredient
tagging with on-hand stock tracking, recipe ratings, and an auto-generated
weekly meal schedule - plus an MCP server so an LLM client can import
recipes and log ingredient stock on your behalf.

Modeled after [local-podcasts](../local-podcasts) (Go/Echo/GORM backend,
plain Create React App frontend, Istio/Pocket ID identity, k8s deployment),
adapted for recipes.

## Architecture

- **`cmd/server`** - the public HTTP API + serves the built React SPA.
- **`cmd/mcpserver`** - a separate process exposing an MCP (Model Context
  Protocol) server over Streamable HTTP, so an LLM client can call
  `import_recipe` / `log_ingredient_stock` / list tools on a user's behalf.
- **`internal/recipes`** - the shared domain package (models, CRUD,
  scheduling algorithm) used by both of the above, so recipe import and
  ingredient logging behave identically whether triggered from the web UI
  or an MCP tool call.
- **`internal/identity`** - resolves the current user from the
  `X-Auth-Request-User` header.
- **`handlers`** - Echo HTTP handlers/routing for `cmd/server`.
- **`app/local-recipes`** - the React frontend (plain Create React App,
  react-router-dom, react-markdown for rendering recipe bodies).

### Data model: single shared PostgreSQL database

Unlike a per-tenant-database design, local-recipes uses **one PostgreSQL
database for every user**, with every table scoped by a `user_id` column
(the same pattern local-podcasts uses for `UserFavorite`). There's no
cross-user leakage risk from a bug in per-user DB provisioning/naming, and
operationally there's just one database to back up/manage (see
`deployment/deployment.yaml`'s CloudNativePG `Cluster`).

See `internal/recipes/models.go` for the full schema:

- `Recipe` - title, description, a Markdown `body_markdown` field (for
  rich instructions/notes), prep/cook time, servings, source/image URLs.
- `Ingredient` / `IngredientStock` - a per-user ingredient catalog, each
  with a tracked on-hand quantity and a configurable low-stock threshold.
- `RecipeIngredient` - tags a recipe with the ingredients (and
  quantities) it calls for.
- `Rating` - one 1-5 rating (+ optional notes) per user per recipe.
- `MealScheduleConfig` / `ScheduledMeal` - a user's meals-per-week /
  meals-to-cook / leftover-days preferences, and the generated weekly
  schedule itself (see "Meal scheduling" below).

### Identity: fails closed, no anonymous fallback

Both `cmd/server` and `cmd/mcpserver` require the `X-Auth-Request-User`
header (as injected by Istio's `ext_authz` AuthorizationPolicy gating
Pocket ID SSO) on every API/tool call, returning `401`/a tool error if
it's missing. There's no "anonymous user" fallback - every row in the
single shared database is partitioned by this user ID, so a missing
identity has no safe bucket to write to.

### Meal scheduling

`POST /api/schedule/generate?week_start=YYYY-MM-DD` (re)generates a week's
plan from a user's `MealScheduleConfig` (meals per week, how many of
those to actually cook vs. eat as leftovers, and how many days a cooked
meal's leftovers last):

1. Picks recipes to cook from the user's library, preferring ones that
   haven't been cooked recently (and, as a tiebreaker, rating).
2. Spreads cook-days evenly across the week.
3. Fills every other day with a leftover reference back to the most
   recent eligible cooked meal, respecting the configured leftover-day
   limit.

Regenerating a week is idempotent - it replaces that week's
`ScheduledMeal` rows rather than accumulating duplicates.

## Running locally

```
make up        # docker compose: Postgres + API server (air hot reload) + MCP server + frontend dev server
make frontend  # just the React dev server, if you'd rather run the backend natively
make seed      # a couple of example recipes, via curl
make down
```

The API server listens on `:8080`, the MCP server on `:8090`, and the
frontend dev server on `:3000` (proxying API requests to the backend -
see `app/local-recipes/package.json`'s `proxy` field).

Without Istio injecting `X-Auth-Request-User` in front of you locally,
set it yourself when calling the API directly, e.g.:

```
curl -H "X-Auth-Request-User: dev-user" http://localhost:8080/api/recipes
```

## Configuration

| Env var | Binary | Default | Description |
|---|---|---|---|
| `LOCAL_RECIPES_DB_DSN` | `cmd/server` | *(required)* | PostgreSQL connection string |
| `LOCAL_RECIPES_ADDR` | `cmd/server` | `:8080` | Listen address |
| `LOCAL_RECIPES_DB_MAX_OPEN_CONNS` | `cmd/server` | `20` | Postgres connection pool size |
| `LOCAL_RECIPES_STATIC_DIR` | `cmd/server` | `static` | Directory the built frontend is served from |
| `LOCAL_RECIPES_MCP_DB_DSN` | `cmd/mcpserver` | *(required)* | PostgreSQL connection string |
| `LOCAL_RECIPES_MCP_ADDR` | `cmd/mcpserver` | `:8090` | Listen address |
| `LOCAL_RECIPES_MCP_DB_MAX_OPEN_CONNS` | `cmd/mcpserver` | `10` | Postgres connection pool size |

Both binaries connect to the same Postgres database and run
`internal/recipes.AutoMigrate` at startup, so either can be the first to
touch a brand-new database.

## MCP server

Point an MCP client (e.g. a chat assistant) at the MCP server's
Streamable HTTP endpoint (`http://<host>:8090/mcp` in this build - see
`cmd/mcpserver/main.go`), with `X-Auth-Request-User` set/injected the same
way it is for the web app. Five tools are available (see
`cmd/mcpserver/tools.go`):

- `import_recipe` - create a recipe (title, Markdown body, ingredients) -
  the same code path `POST /api/recipes` uses, so imported recipes look
  identical to manually-entered ones.
- `log_ingredient_stock` - adjust or set an ingredient's on-hand quantity.
- `list_ingredients` / `list_low_stock_ingredients` - check current stock.
- `list_recipes` - see what's already in the library (e.g. before
  importing a duplicate).

## Deployment

`deployment/deployment.yaml` provisions (per the conventions in the
sibling `k8s-deployment` repo): a Namespace, a single-instance
CloudNativePG `Cluster` ("database") shared by both binaries, Deployments
+ Services for the API server and MCP server, and example Ingresses. Both
Deployments read the Postgres connection string from the CNPG-managed
`database-app` Secret's `uri` key (appending `?sslmode=disable`, matching
local-podcasts/finance-tracker's in-cluster TLS posture).

To actually route traffic through Pocket ID identity gating, add the
chosen hostnames (`recipes.internal.morinapps.com` /
`recipes-mcp.internal.morinapps.com` in the example Ingresses) to the
`pocket-id-sso-internal` `AuthorizationPolicy` in
`k8s-deployment/deployments/networking/pocket-id-sso.yaml` - this repo
intentionally doesn't modify that shared cluster config itself.
