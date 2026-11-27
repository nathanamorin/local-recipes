package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"gorm.io/gorm"
)

// --- import_recipe ---

// importRecipeArgs is the input schema for the import_recipe tool,
// generated automatically via mcp.WithInputSchema[importRecipeArgs]()
// from these struct tags.
type importRecipeArgs struct {
	Title        string                 `json:"title" jsonschema:"required,description=The recipe's title"`
	Description  string                 `json:"description,omitempty" jsonschema:"description=A one or two sentence summary of the dish"`
	BodyMarkdown string                 `json:"body_markdown" jsonschema:"required,description=The full recipe body (ingredient list plus numbered instructions and any notes) formatted as Markdown"`
	Servings     int                    `json:"servings,omitempty" jsonschema:"description=Number of servings this recipe makes"`
	PrepMinutes  int                    `json:"prep_minutes,omitempty" jsonschema:"description=Prep time in minutes"`
	CookMinutes  int                    `json:"cook_minutes,omitempty" jsonschema:"description=Cook time in minutes"`
	SourceURL    string                 `json:"source_url,omitempty" jsonschema:"description=The URL this recipe was imported from, if any"`
	ImageURL     string                 `json:"image_url,omitempty" jsonschema:"description=An image URL for the finished dish, if known"`
	Ingredients  []importIngredientArgs `json:"ingredients" jsonschema:"required,description=The structured ingredient list, used to tag the recipe and cross-reference against stock levels"`
}

type importIngredientArgs struct {
	Name     string  `json:"name" jsonschema:"required,description=Ingredient name, e.g. 'all-purpose flour'"`
	Quantity float64 `json:"quantity,omitempty" jsonschema:"description=Numeric quantity required, e.g. 1.5"`
	Unit     string  `json:"unit,omitempty" jsonschema:"description=Unit for quantity, e.g. 'cup', 'g', 'tbsp'"`
	Notes    string  `json:"notes,omitempty" jsonschema:"description=Free-text preparation note, e.g. 'finely chopped'"`
	Optional bool    `json:"optional,omitempty" jsonschema:"description=True if this ingredient is optional"`
	Category string  `json:"category,omitempty" jsonschema:"description=Ingredient category, e.g. 'produce', 'dairy', 'pantry', 'protein', 'spice'"`
}

func importRecipeTool() mcp.Tool {
	return mcp.NewTool("import_recipe",
		mcp.WithDescription("Import a recipe (e.g. found on the web, or transcribed from a photo/conversation) into the current user's recipe library, with its ingredients tagged so they can be tracked against stock levels. The recipe body should be well-formatted Markdown."),
		mcp.WithInputSchema[importRecipeArgs](),
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

		ingredients := make([]recipes.IngredientInput, len(args.Ingredients))
		for i, ing := range args.Ingredients {
			ingredients[i] = recipes.IngredientInput{
				Name:     ing.Name,
				Quantity: ing.Quantity,
				Unit:     ing.Unit,
				Notes:    ing.Notes,
				Optional: ing.Optional,
				Category: ing.Category,
			}
		}

		recipe, err := recipes.CreateRecipe(db, userID, recipes.RecipeInput{
			Title:        args.Title,
			Description:  args.Description,
			BodyMarkdown: args.BodyMarkdown,
			Servings:     args.Servings,
			PrepMinutes:  args.PrepMinutes,
			CookMinutes:  args.CookMinutes,
			SourceURL:    args.SourceURL,
			ImageURL:     args.ImageURL,
			Ingredients:  ingredients,
		})
		if err != nil {
			return toolError(err)
		}
		return mcp.NewToolResultStructured(recipe, "Imported recipe "+recipe.Title), nil
	}
}

// --- log_ingredient_stock ---

type logIngredientStockArgs struct {
	Ingredient string   `json:"ingredient" jsonschema:"required,description=Ingredient name, e.g. 'eggs'. Created automatically if it doesn't exist yet"`
	Delta      float64  `json:"delta,omitempty" jsonschema:"description=Relative change in quantity on hand: negative after using some up (e.g. -2), positive after buying more (e.g. 12). Mutually exclusive with quantity"`
	Quantity   *float64 `json:"quantity,omitempty" jsonschema:"description=Absolute quantity now on hand (e.g. after a pantry recount). Mutually exclusive with delta"`
	Unit       string   `json:"unit,omitempty" jsonschema:"description=Unit for the quantity, e.g. 'cup', 'count', 'g'"`
}

func logIngredientStockTool() mcp.Tool {
	return mcp.NewTool("log_ingredient_stock",
		mcp.WithDescription("Record a change in how much of an ingredient the current user currently has on hand - either a relative delta (used some, bought more) or an absolute recount. Use this to keep pantry/fridge stock levels accurate as the user mentions using or buying ingredients."),
		mcp.WithInputSchema[logIngredientStockArgs](),
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
		return mcp.NewToolResultStructured(stock, "Updated stock for "+ing.Name), nil
	}
}

// --- list_ingredients ---

type listIngredientsArgs struct{}

func listIngredientsTool() mcp.Tool {
	return mcp.NewTool("list_ingredients",
		mcp.WithDescription("List every ingredient in the current user's catalog along with its current on-hand stock level."),
		mcp.WithInputSchema[listIngredientsArgs](),
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
		return mcp.NewToolResultStructured(list, "Listed ingredients"), nil
	}
}

// --- list_low_stock_ingredients ---

type listLowStockArgs struct{}

func listLowStockTool() mcp.Tool {
	return mcp.NewTool("list_low_stock_ingredients",
		mcp.WithDescription("List ingredients that are at or below their configured low-stock threshold, e.g. to build a grocery list."),
		mcp.WithInputSchema[listLowStockArgs](),
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
		return mcp.NewToolResultStructured(list, "Listed low-stock ingredients"), nil
	}
}

// --- list_recipes ---

type listRecipesArgs struct{}

func listRecipesTool() mcp.Tool {
	return mcp.NewTool("list_recipes",
		mcp.WithDescription("List every recipe in the current user's recipe library, with average rating."),
		mcp.WithInputSchema[listRecipesArgs](),
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
		return mcp.NewToolResultStructured(list, "Listed recipes"), nil
	}
}
