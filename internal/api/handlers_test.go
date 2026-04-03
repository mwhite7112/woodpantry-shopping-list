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
						Name:             "flour",
						Category:         "baking",
						QuantityNeeded:   2,
						QuantityInPantry: 0.5,
						QuantityToBuy:    1.5,
						Unit:             "cup",
						CreatedAt:        time.Unix(101, 0).UTC(),
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
	require.Len(t, resp.Items, 1)
	assert.Equal(t, ingredientID, resp.Items[0].IngredientID)
	assert.Equal(t, 1.5, resp.Items[0].QuantityToBuy)
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
