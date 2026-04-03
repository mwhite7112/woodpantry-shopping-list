package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/logging"
	"github.com/mwhite7112/woodpantry-shopping-list/internal/service"
)

type shoppingListService interface {
	GenerateShoppingList(ctx context.Context, params service.CreateShoppingListParams) (service.ShoppingList, error)
	GetShoppingList(ctx context.Context, id uuid.UUID) (service.ShoppingList, error)
}

func NewRouter(svc shoppingListService) http.Handler {
	router := chi.NewRouter()
	router.Use(logging.Middleware)
	router.Use(middleware.Recoverer)

	router.Get("/healthz", handleHealth)
	router.Post("/shopping-list", handleCreateShoppingList(svc))
	router.Get("/shopping-list/{id}", handleGetShoppingList(svc))

	return router
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("ok")) //nolint:errcheck
}

type createShoppingListRequest struct {
	RecipeIDs  []string `json:"recipe_ids"`
	MealPlanID string   `json:"meal_plan_id"`
}

func handleCreateShoppingList(svc shoppingListService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createShoppingListRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		recipeIDs := make([]uuid.UUID, 0, len(req.RecipeIDs))
		for _, rawID := range req.RecipeIDs {
			id, err := uuid.Parse(rawID)
			if err != nil {
				jsonError(w, "invalid recipe_id", http.StatusBadRequest)
				return
			}
			recipeIDs = append(recipeIDs, id)
		}

		var mealPlanID *uuid.UUID
		if req.MealPlanID != "" {
			parsedMealPlanID, err := uuid.Parse(req.MealPlanID)
			if err != nil {
				jsonError(w, "invalid meal_plan_id", http.StatusBadRequest)
				return
			}
			mealPlanID = &parsedMealPlanID
		}

		list, err := svc.GenerateShoppingList(r.Context(), service.CreateShoppingListParams{
			RecipeIDs:  recipeIDs,
			MealPlanID: mealPlanID,
		})
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}

		jsonWithStatus(w, http.StatusCreated, responseFromShoppingList(list))
	}
}

func handleGetShoppingList(svc shoppingListService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			jsonError(w, "invalid id", http.StatusBadRequest)
			return
		}

		list, err := svc.GetShoppingList(r.Context(), id)
		if err != nil {
			if errors.Is(err, service.ErrShoppingListNotFound) {
				jsonError(w, "shopping list not found", http.StatusNotFound)
				return
			}

			jsonError(w, "failed to get shopping list", http.StatusInternalServerError)
			return
		}

		jsonOK(w, responseFromShoppingList(list))
	}
}

type shoppingListResponse struct {
	ID         uuid.UUID              `json:"id"`
	RecipeIDs  []uuid.UUID            `json:"recipe_ids"`
	MealPlanID *uuid.UUID             `json:"meal_plan_id,omitempty"`
	CreatedAt  string                 `json:"created_at"`
	Items      []shoppingItemResponse `json:"items"`
}

type shoppingItemResponse struct {
	ID               uuid.UUID `json:"id"`
	IngredientID     uuid.UUID `json:"ingredient_id"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	QuantityNeeded   float64   `json:"quantity_needed"`
	QuantityInPantry float64   `json:"quantity_in_pantry"`
	QuantityToBuy    float64   `json:"quantity_to_buy"`
	Unit             string    `json:"unit"`
	CreatedAt        string    `json:"created_at"`
}

func responseFromShoppingList(list service.ShoppingList) shoppingListResponse {
	items := make([]shoppingItemResponse, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, shoppingItemResponse{
			ID:               item.ID,
			IngredientID:     item.IngredientID,
			Name:             item.Name,
			Category:         item.Category,
			QuantityNeeded:   item.QuantityNeeded,
			QuantityInPantry: item.QuantityInPantry,
			QuantityToBuy:    item.QuantityToBuy,
			Unit:             item.Unit,
			CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	return shoppingListResponse{
		ID:         list.ID,
		RecipeIDs:  list.RecipeIDs,
		MealPlanID: list.MealPlanID,
		CreatedAt:  list.CreatedAt.UTC().Format(time.RFC3339),
		Items:      items,
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	jsonWithStatus(w, http.StatusOK, v)
}

func jsonWithStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	jsonWithStatus(w, status, map[string]string{"error": msg})
}
