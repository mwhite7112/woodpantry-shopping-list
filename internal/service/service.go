package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/db"
)

var ErrShoppingListNotFound = errors.New("shopping list not found")

type RecipeClient interface {
	GetRecipe(ctx context.Context, id uuid.UUID) (Recipe, error)
}

type PantryClient interface {
	ListPantry(ctx context.Context) ([]PantryItem, error)
}

type DictionaryClient interface {
	GetIngredient(ctx context.Context, id uuid.UUID) (IngredientDetail, error)
	ListConversions(ctx context.Context, id uuid.UUID) ([]UnitConversion, error)
}

type store interface {
	CreateShoppingList(ctx context.Context, arg db.CreateShoppingListParams) (db.ShoppingList, error)
	CreateShoppingListItem(ctx context.Context, arg db.CreateShoppingListItemParams) (db.ShoppingListItem, error)
	GetShoppingList(ctx context.Context, id uuid.UUID) (db.ShoppingList, error)
	ListShoppingListItems(ctx context.Context, shoppingListID uuid.UUID) ([]db.ShoppingListItem, error)
	WithinTx(ctx context.Context, fn func(q db.Querier) error) error
}

type Service struct {
	store      store
	recipes    RecipeClient
	pantry     PantryClient
	dictionary DictionaryClient
}

type CreateShoppingListParams struct {
	RecipeIDs  []uuid.UUID
	MealPlanID *uuid.UUID
}

type ShoppingList struct {
	ID         uuid.UUID      `json:"id"`
	RecipeIDs  []uuid.UUID    `json:"recipe_ids"`
	MealPlanID *uuid.UUID     `json:"meal_plan_id,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	Items      []ShoppingItem `json:"items"`
}

type ShoppingItem struct {
	ID               uuid.UUID `json:"id"`
	IngredientID     uuid.UUID `json:"ingredient_id"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	QuantityNeeded   float64   `json:"quantity_needed"`
	QuantityInPantry float64   `json:"quantity_in_pantry"`
	QuantityToBuy    float64   `json:"quantity_to_buy"`
	Unit             string    `json:"unit"`
	CreatedAt        time.Time `json:"created_at"`
}

type Recipe struct {
	ID          uuid.UUID
	Ingredients []RecipeIngredient
}

type RecipeIngredient struct {
	IngredientID uuid.UUID
	Quantity     float64
	Unit         string
	IsOptional   bool
}

type PantryItem struct {
	IngredientID uuid.UUID
	Quantity     float64
	Unit         string
}

type IngredientDetail struct {
	ID          uuid.UUID
	Name        string
	Category    string
	DefaultUnit string
}

type UnitConversion struct {
	FromUnit string
	ToUnit   string
	Factor   float64
}

func New(
	sqlDB *sql.DB,
	queries *db.Queries,
	recipes RecipeClient,
	pantry PantryClient,
	dictionary DictionaryClient,
) *Service {
	return newWithStore(&sqlStore{db: sqlDB, queries: queries}, recipes, pantry, dictionary)
}

func newWithStore(
	store store,
	recipes RecipeClient,
	pantry PantryClient,
	dictionary DictionaryClient,
) *Service {
	return &Service{
		store:      store,
		recipes:    recipes,
		pantry:     pantry,
		dictionary: dictionary,
	}
}

func (s *Service) GenerateShoppingList(
	ctx context.Context,
	params CreateShoppingListParams,
) (ShoppingList, error) {
	if len(params.RecipeIDs) == 0 {
		return ShoppingList{}, errors.New("at least one recipe_id is required")
	}

	recipes := make([]Recipe, 0, len(params.RecipeIDs))
	for _, recipeID := range params.RecipeIDs {
		recipe, err := s.recipes.GetRecipe(ctx, recipeID)
		if err != nil {
			return ShoppingList{}, fmt.Errorf("get recipe %s: %w", recipeID, err)
		}
		recipes = append(recipes, recipe)
	}

	pantryItems, err := s.pantry.ListPantry(ctx)
	if err != nil {
		return ShoppingList{}, fmt.Errorf("list pantry: %w", err)
	}

	cache := newIngredientCache(s.dictionary)
	required, err := buildNeededItems(ctx, cache, recipes)
	if err != nil {
		return ShoppingList{}, err
	}
	fillPantryQuantities(ctx, cache, required, pantryItems)

	listID := uuid.New()
	mealPlanID := uuid.NullUUID{}
	if params.MealPlanID != nil {
		mealPlanID = uuid.NullUUID{UUID: *params.MealPlanID, Valid: true}
	}

	items := make([]ShoppingItem, 0, len(required))
	var listRow db.ShoppingList
	err = s.store.WithinTx(ctx, func(q db.Querier) error {
		var txErr error
		listRow, txErr = q.CreateShoppingList(ctx, db.CreateShoppingListParams{
			ID:         listID,
			RecipeIds:  slices.Clone(params.RecipeIDs),
			MealPlanID: mealPlanID,
		})
		if txErr != nil {
			return fmt.Errorf("create shopping list: %w", txErr)
		}

		for _, item := range sortAggregated(required) {
			if item.quantityToBuy <= 0 {
				continue
			}

			row, txErr := q.CreateShoppingListItem(ctx, db.CreateShoppingListItemParams{
				ID:               uuid.New(),
				ShoppingListID:   listRow.ID,
				IngredientID:     item.ingredient.ID,
				Name:             item.ingredient.Name,
				Category:         item.ingredient.Category,
				QuantityNeeded:   item.quantityNeeded,
				QuantityInPantry: item.quantityInPantry,
				QuantityToBuy:    item.quantityToBuy,
				Unit:             item.unit,
			})
			if txErr != nil {
				return fmt.Errorf("create shopping list item: %w", txErr)
			}

			items = append(items, shoppingItemFromRow(row))
		}

		return nil
	})
	if err != nil {
		return ShoppingList{}, err
	}

	return shoppingListFromRows(listRow, items), nil
}

func (s *Service) GetShoppingList(ctx context.Context, id uuid.UUID) (ShoppingList, error) {
	row, err := s.store.GetShoppingList(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ShoppingList{}, ErrShoppingListNotFound
		}
		return ShoppingList{}, fmt.Errorf("get shopping list: %w", err)
	}

	itemRows, err := s.store.ListShoppingListItems(ctx, id)
	if err != nil {
		return ShoppingList{}, fmt.Errorf("list shopping list items: %w", err)
	}

	items := make([]ShoppingItem, 0, len(itemRows))
	for _, itemRow := range itemRows {
		items = append(items, shoppingItemFromRow(itemRow))
	}

	return shoppingListFromRows(row, items), nil
}

type sqlStore struct {
	db      *sql.DB
	queries *db.Queries
}

func (s *sqlStore) CreateShoppingList(
	ctx context.Context,
	arg db.CreateShoppingListParams,
) (db.ShoppingList, error) {
	return s.queries.CreateShoppingList(ctx, arg)
}

func (s *sqlStore) CreateShoppingListItem(
	ctx context.Context,
	arg db.CreateShoppingListItemParams,
) (db.ShoppingListItem, error) {
	return s.queries.CreateShoppingListItem(ctx, arg)
}

func (s *sqlStore) GetShoppingList(ctx context.Context, id uuid.UUID) (db.ShoppingList, error) {
	return s.queries.GetShoppingList(ctx, id)
}

func (s *sqlStore) ListShoppingListItems(
	ctx context.Context,
	shoppingListID uuid.UUID,
) ([]db.ShoppingListItem, error) {
	return s.queries.ListShoppingListItems(ctx, shoppingListID)
}

func (s *sqlStore) WithinTx(ctx context.Context, fn func(q db.Querier) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := fn(db.New(tx)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit shopping list: %w", err)
	}

	return nil
}

type ingredientCache struct {
	dictionary  DictionaryClient
	ingredients map[uuid.UUID]IngredientDetail
	conversions map[uuid.UUID][]UnitConversion
}

func newIngredientCache(dictionary DictionaryClient) *ingredientCache {
	return &ingredientCache{
		dictionary:  dictionary,
		ingredients: make(map[uuid.UUID]IngredientDetail),
		conversions: make(map[uuid.UUID][]UnitConversion),
	}
}

func (c *ingredientCache) getIngredient(ctx context.Context, id uuid.UUID) (IngredientDetail, error) {
	if ingredient, ok := c.ingredients[id]; ok {
		return ingredient, nil
	}

	ingredient, err := c.dictionary.GetIngredient(ctx, id)
	if err != nil {
		return IngredientDetail{}, fmt.Errorf("get ingredient %s: %w", id, err)
	}
	c.ingredients[id] = ingredient
	return ingredient, nil
}

func (c *ingredientCache) getConversions(ctx context.Context, id uuid.UUID) ([]UnitConversion, error) {
	if conversions, ok := c.conversions[id]; ok {
		return conversions, nil
	}

	conversions, err := c.dictionary.ListConversions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list conversions %s: %w", id, err)
	}
	c.conversions[id] = conversions
	return conversions, nil
}

type aggregatedItem struct {
	ingredient       IngredientDetail
	unit             string
	quantityNeeded   float64
	quantityInPantry float64
	quantityToBuy    float64
}

func buildNeededItems(
	ctx context.Context,
	cache *ingredientCache,
	recipes []Recipe,
) (map[string]*aggregatedItem, error) {
	required := make(map[string]*aggregatedItem)

	for _, recipe := range recipes {
		for _, ingredient := range recipe.Ingredients {
			if ingredient.IsOptional || ingredient.Quantity <= 0 {
				continue
			}

			meta, err := cache.getIngredient(ctx, ingredient.IngredientID)
			if err != nil {
				return nil, err
			}

			conversions, err := cache.getConversions(ctx, ingredient.IngredientID)
			if err != nil {
				return nil, err
			}

			quantity, unit := normalizeLineItem(ingredient.Quantity, ingredient.Unit, meta.DefaultUnit, conversions)
			key := aggregateKey(ingredient.IngredientID, unit)

			item, ok := required[key]
			if !ok {
				item = &aggregatedItem{
					ingredient: meta,
					unit:       unit,
				}
				required[key] = item
			}

			item.quantityNeeded += quantity
			item.quantityToBuy = item.quantityNeeded
		}
	}

	return required, nil
}

func fillPantryQuantities(
	ctx context.Context,
	cache *ingredientCache,
	required map[string]*aggregatedItem,
	pantryItems []PantryItem,
) {
	for _, pantryItem := range pantryItems {
		if pantryItem.Quantity <= 0 {
			continue
		}

		meta, err := cache.getIngredient(ctx, pantryItem.IngredientID)
		if err != nil {
			continue
		}

		conversions, err := cache.getConversions(ctx, pantryItem.IngredientID)
		if err != nil {
			continue
		}

		quantity, unit := normalizeLineItem(pantryItem.Quantity, pantryItem.Unit, meta.DefaultUnit, conversions)
		key := aggregateKey(pantryItem.IngredientID, unit)
		item, ok := required[key]
		if !ok {
			continue
		}

		item.quantityInPantry += quantity
		item.quantityToBuy = maxFloat(item.quantityNeeded-item.quantityInPantry, 0)
	}
}

func normalizeLineItem(
	quantity float64,
	unit string,
	defaultUnit string,
	conversions []UnitConversion,
) (float64, string) {
	normalizedUnit := normalizeUnit(unit)
	targetUnit := normalizeUnit(defaultUnit)
	if normalizedUnit == "" {
		return quantity, targetUnit
	}
	if targetUnit == "" || targetUnit == normalizedUnit {
		return quantity, normalizedUnit
	}

	convertedQuantity, ok := convertQuantity(quantity, normalizedUnit, targetUnit, conversions)
	if !ok {
		return quantity, normalizedUnit
	}

	return convertedQuantity, targetUnit
}

func convertQuantity(
	quantity float64,
	fromUnit string,
	toUnit string,
	conversions []UnitConversion,
) (float64, bool) {
	from := normalizeUnit(fromUnit)
	to := normalizeUnit(toUnit)
	if from == "" || to == "" {
		return quantity, false
	}
	if from == to {
		return quantity, true
	}

	type edge struct {
		unit   string
		factor float64
	}

	graph := make(map[string][]edge)
	for _, conversion := range conversions {
		fromConv := normalizeUnit(conversion.FromUnit)
		toConv := normalizeUnit(conversion.ToUnit)
		if fromConv == "" || toConv == "" || conversion.Factor == 0 {
			continue
		}

		graph[fromConv] = append(graph[fromConv], edge{unit: toConv, factor: conversion.Factor})
		graph[toConv] = append(graph[toConv], edge{unit: fromConv, factor: 1 / conversion.Factor})
	}

	type node struct {
		unit   string
		factor float64
	}

	queue := []node{{unit: from, factor: 1}}
	visited := map[string]bool{from: true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range graph[current.unit] {
			if visited[next.unit] {
				continue
			}

			factor := current.factor * next.factor
			if next.unit == to {
				return quantity * factor, true
			}

			visited[next.unit] = true
			queue = append(queue, node{unit: next.unit, factor: factor})
		}
	}

	return quantity, false
}

func aggregateKey(ingredientID uuid.UUID, unit string) string {
	return ingredientID.String() + "|" + normalizeUnit(unit)
}

func normalizeUnit(unit string) string {
	return strings.ToLower(strings.TrimSpace(unit))
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func sortAggregated(items map[string]*aggregatedItem) []*aggregatedItem {
	sorted := make([]*aggregatedItem, 0, len(items))
	for _, item := range items {
		sorted = append(sorted, item)
	}

	slices.SortFunc(sorted, func(a *aggregatedItem, b *aggregatedItem) int {
		if cmp := strings.Compare(a.ingredient.Category, b.ingredient.Category); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.ingredient.Name, b.ingredient.Name); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.unit, b.unit)
	})

	return sorted
}

func shoppingListFromRows(row db.ShoppingList, items []ShoppingItem) ShoppingList {
	var mealPlanID *uuid.UUID
	if row.MealPlanID.Valid {
		mealPlanID = &row.MealPlanID.UUID
	}

	return ShoppingList{
		ID:         row.ID,
		RecipeIDs:  slices.Clone(row.RecipeIds),
		MealPlanID: mealPlanID,
		CreatedAt:  row.CreatedAt,
		Items:      items,
	}
}

func shoppingItemFromRow(row db.ShoppingListItem) ShoppingItem {
	return ShoppingItem{
		ID:               row.ID,
		IngredientID:     row.IngredientID,
		Name:             row.Name,
		Category:         row.Category,
		QuantityNeeded:   row.QuantityNeeded,
		QuantityInPantry: row.QuantityInPantry,
		QuantityToBuy:    row.QuantityToBuy,
		Unit:             row.Unit,
		CreatedAt:        row.CreatedAt,
	}
}
