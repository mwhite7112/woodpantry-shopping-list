package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/service"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	router := NewRouter(stubShoppingListService{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestCreateShoppingList(t *testing.T) {
	t.Parallel()

	listID := uuid.New()
	recipeID := uuid.New()
	ingredientID := uuid.New()

	router := NewRouter(stubShoppingListService{
		generateFn: func(_ context.Context, params service.CreateShoppingListParams) (service.ShoppingList, error) {
			require.Equal(t, []uuid.UUID{recipeID}, params.RecipeIDs)
			return service.ShoppingList{
				ID:        listID,
				RecipeIDs: []uuid.UUID{recipeID},
				CreatedAt: time.Unix(100, 0).UTC(),
				Items: []service.ShoppingItem{
					{
						ID:               uuid.New(),
						IngredientID:     ingredientID,
						Name:             "banana",
						Category:         "produce",
						QuantityNeeded:   2,
						QuantityInPantry: 0.5,
						QuantityToBuy:    1.5,
						Unit:             "each",
						CreatedAt:        time.Unix(101, 0).UTC(),
					},
					{
						ID:               uuid.New(),
						IngredientID:     uuid.New(),
						Name:             "apple",
						Category:         "produce",
						QuantityNeeded:   3,
						QuantityInPantry: 1,
						QuantityToBuy:    2,
						Unit:             "each",
						CreatedAt:        time.Unix(102, 0).UTC(),
					},
					{
						ID:               uuid.New(),
						IngredientID:     uuid.New(),
						Name:             "flour",
						Category:         "baking",
						QuantityNeeded:   4,
						QuantityInPantry: 0,
						QuantityToBuy:    4,
						Unit:             "cup",
						CreatedAt:        time.Unix(103, 0).UTC(),
					},
				},
			}, nil
		},
	})

	body := bytes.NewBufferString(`{"recipe_ids":["` + recipeID.String() + `"]}`)
	req := httptest.NewRequest(http.MethodPost, "/shopping-list", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp shoppingListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, listID, resp.ID)
	require.Len(t, resp.Items, 3)
	assert.Equal(t, ingredientID, resp.Items[0].IngredientID)
	assert.Equal(t, "banana", resp.Items[0].Name)
	assert.InDelta(t, 1.5, resp.Items[0].QuantityToBuy, 0.0001)

	require.Len(t, resp.Groups, 2)
	assert.Equal(t, "baking", resp.Groups[0].Category)
	require.Len(t, resp.Groups[0].Items, 1)
	assert.Equal(t, "flour", resp.Groups[0].Items[0].Name)
	assert.Equal(t, "produce", resp.Groups[1].Category)
	require.Len(t, resp.Groups[1].Items, 2)
	assert.Equal(t, "apple", resp.Groups[1].Items[0].Name)
	assert.Equal(t, "banana", resp.Groups[1].Items[1].Name)
}

func TestGetShoppingList(t *testing.T) {
	t.Parallel()

	listID := uuid.New()
	router := NewRouter(stubShoppingListService{
		getFn: func(_ context.Context, id uuid.UUID) (service.ShoppingList, error) {
			require.Equal(t, listID, id)
			return service.ShoppingList{
				ID:        listID,
				RecipeIDs: []uuid.UUID{uuid.New()},
				CreatedAt: time.Unix(100, 0).UTC(),
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/shopping-list/"+listID.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp shoppingListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, listID, resp.ID)
}

func TestGetShoppingListNotFound(t *testing.T) {
	t.Parallel()

	router := NewRouter(stubShoppingListService{
		getFn: func(_ context.Context, _ uuid.UUID) (service.ShoppingList, error) {
			return service.ShoppingList{}, service.ErrShoppingListNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/shopping-list/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

type stubShoppingListService struct {
	generateFn func(ctx context.Context, params service.CreateShoppingListParams) (service.ShoppingList, error)
	getFn      func(ctx context.Context, id uuid.UUID) (service.ShoppingList, error)
}

func (s stubShoppingListService) GenerateShoppingList(
	ctx context.Context,
	params service.CreateShoppingListParams,
) (service.ShoppingList, error) {
	if s.generateFn != nil {
		return s.generateFn(ctx, params)
	}
	return service.ShoppingList{}, errors.New("unexpected GenerateShoppingList call")
}

func (s stubShoppingListService) GetShoppingList(
	ctx context.Context,
	id uuid.UUID,
) (service.ShoppingList, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return service.ShoppingList{}, errors.New("unexpected GetShoppingList call")
}
