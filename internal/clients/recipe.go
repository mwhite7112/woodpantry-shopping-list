package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/service"
)

type RecipeClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewRecipeClient(baseURL string, httpClient *http.Client) *RecipeClient {
	return &RecipeClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

type recipeResponse struct {
	ID          uuid.UUID `json:"id"`
	Ingredients []struct {
		IngredientID uuid.UUID `json:"ingredient_id"`
		Quantity     float64   `json:"quantity"`
		Unit         string    `json:"unit"`
		IsOptional   bool      `json:"is_optional"`
	} `json:"ingredients"`
}

func (c *RecipeClient) GetRecipe(ctx context.Context, id uuid.UUID) (service.Recipe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/recipes/"+id.String(), nil)
	if err != nil {
		return service.Recipe{}, fmt.Errorf("recipes get recipe: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return service.Recipe{}, fmt.Errorf("recipes get recipe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return service.Recipe{}, fmt.Errorf("recipes get recipe: unexpected status %d", resp.StatusCode)
	}

	var decoded recipeResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return service.Recipe{}, fmt.Errorf("recipes get recipe decode: %w", err)
	}

	ingredients := make([]service.RecipeIngredient, 0, len(decoded.Ingredients))
	for _, ingredient := range decoded.Ingredients {
		ingredients = append(ingredients, service.RecipeIngredient{
			IngredientID: ingredient.IngredientID,
			Quantity:     ingredient.Quantity,
			Unit:         ingredient.Unit,
			IsOptional:   ingredient.IsOptional,
		})
	}

	return service.Recipe{
		ID:          decoded.ID,
		Ingredients: ingredients,
	}, nil
}
