-- name: CreateShoppingList :one
INSERT INTO shopping_lists (
    id,
    recipe_ids,
    meal_plan_id
) VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: GetShoppingList :one
SELECT *
FROM shopping_lists
WHERE id = $1;

-- name: CreateShoppingListItem :one
INSERT INTO shopping_list_items (
    id,
    shopping_list_id,
    ingredient_id,
    name,
    category,
    quantity_needed,
    quantity_in_pantry,
    quantity_to_buy,
    unit
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
RETURNING *;

-- name: ListShoppingListItems :many
SELECT *
FROM shopping_list_items
WHERE shopping_list_id = $1
ORDER BY category, name, created_at, id;
