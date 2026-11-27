package recipes

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpsertRating sets userID's rating (1-5, with optional notes) for a
// recipe, creating or overwriting their existing rating - see Rating's
// doc comment for why this is "latest rating wins" rather than a review
// history.
func UpsertRating(db *gorm.DB, userID, recipeID string, score int, notes string) (*Rating, error) {
	if score < 1 || score > 5 {
		return nil, fmt.Errorf("score must be between 1 and 5")
	}

	rating := Rating{
		ID:        uuid.NewString(),
		UserID:    userID,
		RecipeID:  recipeID,
		Score:     score,
		Notes:     notes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "recipe_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"score", "notes", "updated_at"}),
	}).Create(&rating).Error
	if err != nil {
		return nil, err
	}
	return &rating, nil
}

// GetRating returns userID's current rating for a recipe, if any.
func GetRating(db *gorm.DB, userID, recipeID string) (*Rating, error) {
	var rating Rating
	err := db.Where("user_id = ? AND recipe_id = ?", userID, recipeID).First(&rating).Error
	if err != nil {
		return nil, err
	}
	return &rating, nil
}
