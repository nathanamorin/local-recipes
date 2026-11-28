package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"gorm.io/gorm"
)

// --- import_recipe ---

// importRecipeArgs is the input schema for the import_recipe tool,
// generated via mcp.WithInputSchema[importRecipeArgs]() using
// google/jsonschema-go conventions: the whole `jsonschema` tag is the
// field description (no "required,"/"description=" keys - those make
// schema generation fail and publish an empty schema), and a field is
// required unless its json tag has omitempty.
type importRecipeArgs struct {
	Title        string                 `json:"title" jsonschema:"The recipe's title"`
	Description  string                 `json:"description,omitempty" jsonschema:"A one or two sentence summary of the dish"`
	BodyMarkdown string                 `json:"body_markdown" jsonschema:"The full recipe body (ingredient list plus numbered instructions and any notes) formatted as Markdown"`
	Servings     int                    `json:"servings,omitempty" jsonschema:"Number of servings this recipe makes"`
	PrepMinutes  int                    `json:"prep_minutes,omitempty" jsonschema:"Prep time in minutes"`
	CookMinutes  int                    `json:"cook_minutes,omitempty" jsonschema:"Cook time in minutes"`
	SourceURL    string                 `json:"source_url,omitempty" jsonschema:"The URL this recipe was imported from, if any"`
	ImageURL     string                 `json:"image_url,omitempty" jsonschema:"An image URL for the finished dish, if known"`
	Ingredients  []importIngredientArgs `json:"ingredients" jsonschema:"The structured ingredient list, used to tag the recipe and cross-reference against stock levels"`
}

type importIngredientArgs struct {
	Name     string  `json:"name" jsonschema:"Ingredient name, e.g. 'all-purpose flour'"`
	Quantity float64 `json:"quantity,omitempty" jsonschema:"Numeric quantity required, e.g. 1.5"`
	Unit     string  `json:"unit,omitempty" jsonschema:"Unit for quantity, e.g. 'cup', 'g', 'tbsp'"`
	Notes    string  `json:"notes,omitempty" jsonschema:"Free-text preparation note, e.g. 'finely chopped'"`
	Optional bool    `json:"optional,omitempty" jsonschema:"True if this ingredient is optional"`
	Category string  `json:"category,omitempty" jsonschema:"Ingredient category, e.g. 'produce', 'dairy', 'pantry', 'protein', 'spice'"`
}

func (a importRecipeArgs) validate() error {
	var missing []string
	if strings.TrimSpace(a.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(a.BodyMarkdown) == "" {
		missing = append(missing, "body_markdown")
	}
	if len(a.Ingredients) == 0 {
		missing = append(missing, "ingredients")
	}
	for i, ing := range a.Ingredients {
		if strings.TrimSpace(ing.Name) == "" {
			missing = append(missing, fmt.Sprintf("ingredients[%d].name", i))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required field(s): %s - nothing was saved", strings.Join(missing, ", "))
	}
	return nil
}

func (a importRecipeArgs) toInput() recipes.RecipeInput {
	ingredients := make([]recipes.IngredientInput, len(a.Ingredients))
	for i, ing := range a.Ingredients {
		ingredients[i] = recipes.IngredientInput{
			Name:     ing.Name,
			Quantity: ing.Quantity,
			Unit:     ing.Unit,
			Notes:    ing.Notes,
			Optional: ing.Optional,
			Category: ing.Category,
		}
	}
	return recipes.RecipeInput{
		Title:        a.Title,
		Description:  a.Description,
		BodyMarkdown: a.BodyMarkdown,
		Servings:     a.Servings,
		PrepMinutes:  a.PrepMinutes,
		CookMinutes:  a.CookMinutes,
		SourceURL:    a.SourceURL,
		ImageURL:     a.ImageURL,
		Ingredients:  ingredients,
	}
}

func importRecipeTool() mcp.Tool {
	return mcp.NewTool("import_recipe",
		mcp.WithDescription("Import a recipe (e.g. found on the web, or transcribed from a photo/conversation) into the current user's recipe library, with its ingredients tagged so they can be tracked against stock levels. The recipe body should be well-formatted Markdown."),
		mcp.WithInputSchema[importRecipeArgs](),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

func importRecipeHandler(db *gorm.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := userIDFromContext(ctx)
		if err != nil {
			return toolError(err)
		}

		var args importRecipeArgs
		if err := request.BindArguments(&args); err != nil {
			return toolError(err)
		}
		if err := args.validate(); err != nil {
			return toolError(err)
		}

		recipe, err := recipes.CreateRecipe(db, userID, args.toInput())
		if err != nil {
			return toolError(err)
		}
		return structuredResult(recipe)
	}
}

// --- update_recipe ---

// updateRecipeArgs replaces an existing recipe wholesale: every field of
// import_recipe, plus the ID of the recipe to overwrite.
type updateRecipeArgs struct {
	RecipeID string `json:"recipe_id" jsonschema:"ID of the recipe to replace, from list_recipes"`
	importRecipeArgs
}

func updateRecipeTool() mcp.Tool {
	return mcp.NewTool("update_recipe",
		mcp.WithDescription("Replace an existing recipe in the current user's library (title, body, metadata and the full ingredient list) - e.g. to fix or complete a recipe. Fields not provided are cleared, so always send the complete recipe. Get the recipe_id from list_recipes."),
		mcp.WithInputSchema[updateRecipeArgs](),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

func updateRecipeHandler(db *gorm.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := userIDFromContext(ctx)
		if err != nil {
			return toolError(err)
		}
		var args updateRecipeArgs
		if err := request.BindArguments(&args); err != nil {
			return toolError(err)
		}
		if strings.TrimSpace(args.RecipeID) == "" {
			return toolError(fmt.Errorf("missing required field(s): recipe_id - nothing was saved"))
		}
		if err := args.validate(); err != nil {
			return toolError(err)
		}
		if _, err := recipes.UpdateRecipe(db, userID, args.RecipeID, args.toInput()); err != nil {
			return toolError(err)
		}
		// Re-read so the result includes resolved ingredients, like import_recipe.
		recipe, err := recipes.GetRecipe(db, userID, args.RecipeID)
		if err != nil {
			return toolError(err)
		}
		return structuredResult(recipe)
	}
}

// --- get_shopping_list ---

type shoppingListArgs struct {
	From string `json:"from,omitempty" jsonschema:"First day to cover, YYYY-MM-DD. Defaults to today"`
	To   string `json:"to,omitempty" jsonschema:"Last day to cover, YYYY-MM-DD. Defaults to the Sunday ending that week"`
}

func shoppingListTool() mcp.Tool {
	return mcp.NewTool("get_shopping_list",
		mcp.WithDescription("Get what the current user needs to buy: ingredients for the meals planned in the date range (minus what's in the pantry, converting units where exact) plus restocking anything at or below its low-stock threshold. Items with tracked=false aren't tracked in the pantry, so the user should check whether they have them."),
		mcp.WithInputSchema[shoppingListArgs](),
		readOnly(),
	)
}

func shoppingListHandler(db *gorm.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := userIDFromContext(ctx)
		if err != nil {
			return toolError(err)
		}
		var args shoppingListArgs
		if err := request.BindArguments(&args); err != nil {
			return toolError(err)
		}
		today := time.Now().UTC().Truncate(24 * time.Hour)
		from, to := today, today
		if args.From != "" {
			if from, err = time.Parse("2006-01-02", args.From); err != nil {
				return toolError(fmt.Errorf("from must be YYYY-MM-DD"))
			}
		}
		to = from.AddDate(0, 0, (7-int(from.Weekday()))%7)
		if args.To != "" {
			if to, err = time.Parse("2006-01-02", args.To); err != nil {
				return toolError(fmt.Errorf("to must be YYYY-MM-DD"))
			}
		}
		list, err := recipes.BuildShoppingList(db, userID, from, to)
		if err != nil {
			return toolError(err)
		}
		return structuredResult(list)
	}
}

// --- cook_recipe ---

type cookRecipeArgs struct {
	RecipeID string `json:"recipe_id" jsonschema:"ID of the recipe that was cooked, from list_recipes"`
	Servings int    `json:"servings,omitempty" jsonschema:"Servings actually made, to scale the amounts. Omit to use the recipe as written"`
	DryRun   bool   `json:"dry_run,omitempty" jsonschema:"If true, only report what would be deducted"`
}

func cookRecipeTool() mcp.Tool {
	return mcp.NewTool("cook_recipe",
		mcp.WithDescription("Record that the user cooked a recipe: deducts one batch of its non-optional ingredients (scaled to servings) from tracked pantry stock. Untracked, optional or unit-incompatible ingredients are skipped and reported."),
		mcp.WithInputSchema[cookRecipeArgs](),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

func cookRecipeHandler(db *gorm.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := userIDFromContext(ctx)
		if err != nil {
			return toolError(err)
		}
		var args cookRecipeArgs
		if err := request.BindArguments(&args); err != nil {
			return toolError(err)
		}
		if strings.TrimSpace(args.RecipeID) == "" {
			return toolError(fmt.Errorf("missing required field(s): recipe_id"))
		}
		res, err := recipes.CookRecipe(db, userID, args.RecipeID, args.Servings, args.DryRun)
		if err != nil {
			return toolError(err)
		}
		return structuredResult(res)
	}
}

// --- log_ingredient_stock ---

type logIngredientStockArgs struct {
	Ingredient string   `json:"ingredient" jsonschema:"Ingredient name, e.g. 'eggs'. Created automatically if it doesn't exist yet"`
	Delta      float64  `json:"delta,omitempty" jsonschema:"Relative change in quantity on hand: negative after using some up (e.g. -2), positive after buying more (e.g. 12). Mutually exclusive with quantity"`
	Quantity   *float64 `json:"quantity,omitempty" jsonschema:"Absolute quantity now on hand (e.g. after a pantry recount). Mutually exclusive with delta"`
	Unit       string   `json:"unit,omitempty" jsonschema:"Unit for the quantity, e.g. 'cup', 'count', 'g'"`
}

func logIngredientStockTool() mcp.Tool {
	return mcp.NewTool("log_ingredient_stock",
		mcp.WithDescription("Record a change in how much of an ingredient the current user currently has on hand - either a relative delta (used some, bought more) or an absolute recount. Use this to keep pantry/fridge stock levels accurate as the user mentions using or buying ingredients."),
		mcp.WithInputSchema[logIngredientStockArgs](),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

func logIngredientStockHandler(db *gorm.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := userIDFromContext(ctx)
		if err != nil {
			return toolError(err)
		}
		var args logIngredientStockArgs
		if err := request.BindArguments(&args); err != nil {
			return toolError(err)
		}

		ing, err := recipes.FindOrCreateIngredient(db, userID, args.Ingredient, "", args.Unit)
		if err != nil {
			return toolError(err)
		}

		var stock *recipes.IngredientStock
		if args.Quantity != nil {
			stock, err = recipes.SetStock(db, userID, ing.ID, *args.Quantity, args.Unit)
		} else {
			stock, err = recipes.AdjustStock(db, userID, ing.ID, args.Delta, args.Unit)
		}
		if err != nil {
			return toolError(err)
		}
		return structuredResult(stock)
	}
}

// --- list_ingredients ---

type listIngredientsArgs struct{}

func listIngredientsTool() mcp.Tool {
	return mcp.NewTool("list_ingredients",
		mcp.WithDescription("List every ingredient in the current user's catalog along with its current on-hand stock level."),
		mcp.WithInputSchema[listIngredientsArgs](),
		readOnly(),
	)
}

func listIngredientsHandler(db *gorm.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := userIDFromContext(ctx)
		if err != nil {
			return toolError(err)
		}
		list, err := recipes.ListIngredients(db, userID)
		if err != nil {
			return toolError(err)
		}
		return structuredResult(map[string]any{"ingredients": list})
	}
}

// --- list_low_stock_ingredients ---

type listLowStockArgs struct{}

func listLowStockTool() mcp.Tool {
	return mcp.NewTool("list_low_stock_ingredients",
		mcp.WithDescription("List ingredients that are at or below their configured low-stock threshold, e.g. to build a grocery list."),
		mcp.WithInputSchema[listLowStockArgs](),
		readOnly(),
	)
}

func listLowStockHandler(db *gorm.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := userIDFromContext(ctx)
		if err != nil {
			return toolError(err)
		}
		list, err := recipes.ListLowStock(db, userID)
		if err != nil {
			return toolError(err)
		}
		return structuredResult(map[string]any{"ingredients": list})
	}
}

// --- list_recipes ---

type listRecipesArgs struct{}

func listRecipesTool() mcp.Tool {
	return mcp.NewTool("list_recipes",
		mcp.WithDescription("List every recipe in the current user's recipe library, with average rating."),
		mcp.WithInputSchema[listRecipesArgs](),
		readOnly(),
	)
}

func listRecipesHandler(db *gorm.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := userIDFromContext(ctx)
		if err != nil {
			return toolError(err)
		}
		list, err := recipes.ListRecipes(db, userID)
		if err != nil {
			return toolError(err)
		}
		return structuredResult(map[string]any{"recipes": list})
	}
}

func readOnly() mcp.ToolOption {
	return func(t *mcp.Tool) {
		mcp.WithReadOnlyHintAnnotation(true)(t)
		mcp.WithDestructiveHintAnnotation(false)(t)
		mcp.WithIdempotentHintAnnotation(true)(t)
		mcp.WithOpenWorldHintAnnotation(false)(t)
	}
}

// structuredResult returns v as structuredContent (which the MCP spec
// requires to be a JSON object - wrap slices before calling) and also as
// the text block, for clients that only read text content.
func structuredResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return toolError(err)
	}
	return mcp.NewToolResultStructured(v, string(b)), nil
}
