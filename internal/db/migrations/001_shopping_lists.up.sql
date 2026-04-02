CREATE TABLE IF NOT EXISTS shopping_lists (
    id UUID PRIMARY KEY,
    recipe_ids UUID[] NOT NULL DEFAULT '{}',
    meal_plan_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shopping_list_items (
    id UUID PRIMARY KEY,
    shopping_list_id UUID NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    quantity_needed DOUBLE PRECISION NOT NULL,
    quantity_in_pantry DOUBLE PRECISION NOT NULL,
    quantity_to_buy DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS shopping_list_items_shopping_list_id_idx
    ON shopping_list_items (shopping_list_id);

CREATE INDEX IF NOT EXISTS shopping_list_items_ingredient_id_idx
    ON shopping_list_items (ingredient_id);
