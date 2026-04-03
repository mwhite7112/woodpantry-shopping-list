package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/db"
)

func TestConvertQuantity(t *testing.T) {
	t.Parallel()

	conversions := []UnitConversion{
		{FromUnit: "cup", ToUnit: "tbsp", Factor: 16},
		{FromUnit: "tbsp", ToUnit: "tsp", Factor: 3},
	}

	quantity, ok := convertQuantity(2, "cup", "tsp", conversions)
	require.True(t, ok)
	assert.Equal(t, 96.0, quantity)

	quantity, ok = convertQuantity(48, "tsp", "cup", conversions)
	require.True(t, ok)
	assert.InDelta(t, 1, quantity, 0.0001)
}

func TestNormalizeLineItemFallsBackWhenNoConversionExists(t *testing.T) {
	t.Parallel()

	quantity, unit := normalizeLineItem(2, "can", "cup", []UnitConversion{
		{FromUnit: "cup", ToUnit: "tbsp", Factor: 16},
	})

	assert.Equal(t, 2.0, quantity)
	assert.Equal(t, "can", unit)
}

func TestGenerateShoppingListAggregatesAndDiffsUsingConversions(t *testing.T) {
	t.Parallel()

	flourID := uuid.New()
	recipeOneID := uuid.New()
	recipeTwoID := uuid.New()
	listID := uuid.New()
	now := time.Now().UTC()

	queries := &stubQuerier{
		createShoppingListFn: func(_ context.Context, arg db.CreateShoppingListParams) (db.ShoppingList, error) {
			return db.ShoppingList{
				ID:        listID,
				RecipeIds: arg.RecipeIds,
				CreatedAt: now,
			}, nil
		},
		createShoppingListItemFn: func(_ context.Context, arg db.CreateShoppingListItemParams) (db.ShoppingListItem, error) {
			return db.ShoppingListItem{
				ID:               arg.ID,
				ShoppingListID:   arg.ShoppingListID,
				IngredientID:     arg.IngredientID,
				Name:             arg.Name,
				Category:         arg.Category,
				QuantityNeeded:   arg.QuantityNeeded,
				QuantityInPantry: arg.QuantityInPantry,
				QuantityToBuy:    arg.QuantityToBuy,
				Unit:             arg.Unit,
				CreatedAt:        now,
			}, nil
		},
	}

	svc := newWithStore(
		queries,
		stubRecipeClient{
			recipes: map[uuid.UUID]Recipe{
				recipeOneID: {
					ID: recipeOneID,
					Ingredients: []RecipeIngredient{
						{IngredientID: flourID, Quantity: 1, Unit: "cup"},
					},
				},
				recipeTwoID: {
					ID: recipeTwoID,
					Ingredients: []RecipeIngredient{
						{IngredientID: flourID, Quantity: 8, Unit: "tbsp"},
					},
				},
			},
		},
		stubPantryClient{
			items: []PantryItem{
				{IngredientID: flourID, Quantity: 4, Unit: "tbsp"},
			},
		},
		stubDictionaryClient{
			ingredients: map[uuid.UUID]IngredientDetail{
				flourID: {
					ID:          flourID,
					Name:        "flour",
					Category:    "baking",
					DefaultUnit: "cup",
				},
			},
			conversions: map[uuid.UUID][]UnitConversion{
				flourID: {
					{FromUnit: "cup", ToUnit: "tbsp", Factor: 16},
				},
			},
		},
	)

	list, err := svc.GenerateShoppingList(context.Background(), CreateShoppingListParams{
		RecipeIDs: []uuid.UUID{recipeOneID, recipeTwoID},
	})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)

	item := list.Items[0]
	assert.Equal(t, flourID, item.IngredientID)
	assert.Equal(t, "flour", item.Name)
	assert.Equal(t, "baking", item.Category)
	assert.Equal(t, "cup", item.Unit)
	assert.InDelta(t, 1.5, item.QuantityNeeded, 0.0001)
	assert.InDelta(t, 0.25, item.QuantityInPantry, 0.0001)
	assert.InDelta(t, 1.25, item.QuantityToBuy, 0.0001)
}

type stubRecipeClient struct {
	recipes map[uuid.UUID]Recipe
}

func (s stubRecipeClient) GetRecipe(_ context.Context, id uuid.UUID) (Recipe, error) {
	return s.recipes[id], nil
}

type stubPantryClient struct {
	items []PantryItem
}

func (s stubPantryClient) ListPantry(_ context.Context) ([]PantryItem, error) {
	return s.items, nil
}

type stubDictionaryClient struct {
	ingredients map[uuid.UUID]IngredientDetail
	conversions map[uuid.UUID][]UnitConversion
}

func (s stubDictionaryClient) GetIngredient(_ context.Context, id uuid.UUID) (IngredientDetail, error) {
	return s.ingredients[id], nil
}

func (s stubDictionaryClient) ListConversions(_ context.Context, id uuid.UUID) ([]UnitConversion, error) {
	return s.conversions[id], nil
}

type stubQuerier struct {
	createShoppingListFn     func(ctx context.Context, arg db.CreateShoppingListParams) (db.ShoppingList, error)
	createShoppingListItemFn func(ctx context.Context, arg db.CreateShoppingListItemParams) (db.ShoppingListItem, error)
	getShoppingListFn        func(ctx context.Context, id uuid.UUID) (db.ShoppingList, error)
	listShoppingListItemsFn  func(ctx context.Context, shoppingListID uuid.UUID) ([]db.ShoppingListItem, error)
}

func (s *stubQuerier) WithinTx(_ context.Context, fn func(q db.Querier) error) error {
	return fn(s)
}

func (s *stubQuerier) CreateShoppingList(ctx context.Context, arg db.CreateShoppingListParams) (db.ShoppingList, error) {
	return s.createShoppingListFn(ctx, arg)
}

func (s *stubQuerier) CreateShoppingListItem(
	ctx context.Context,
	arg db.CreateShoppingListItemParams,
) (db.ShoppingListItem, error) {
	return s.createShoppingListItemFn(ctx, arg)
}

func (s *stubQuerier) GetShoppingList(ctx context.Context, id uuid.UUID) (db.ShoppingList, error) {
	return s.getShoppingListFn(ctx, id)
}

func (s *stubQuerier) ListShoppingListItems(
	ctx context.Context,
	shoppingListID uuid.UUID,
) ([]db.ShoppingListItem, error) {
	return s.listShoppingListItemsFn(ctx, shoppingListID)
}
