# woodpantry-shopping-list — Shopping List Service

## Role in Architecture

Given a target set of recipes, this service generates a persisted shopping list by aggregating recipe ingredient requirements, diffing them against the current pantry, and normalizing units with Ingredient Dictionary conversion data when available.

This service is Phase 2. It is synchronous and read-heavy, with writes only for persisting generated shopping lists and their items.

## Technology

- Language: Go
- HTTP: chi
- Database: PostgreSQL (`shopping_list_db`) via sqlc
- No RabbitMQ

## Service Dependencies

- Calls Recipe Service: `GET /recipes/{id}`
- Calls Pantry Service: `GET /pantry`
- Calls Ingredient Dictionary: `GET /ingredients/{id}`
- Calls Ingredient Dictionary: `GET /ingredients/{id}/conversions`
- Called by: frontend or any orchestration layer that needs a persisted generated list

## Implemented API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check |
| POST | `/shopping-list` | Generate and persist a shopping list from recipe IDs |
| GET | `/shopping-list/:id` | Fetch a saved shopping list |

### POST /shopping-list

Request:

```json
{
  "recipe_ids": ["uuid", "uuid"],
  "meal_plan_id": "uuid-optional"
}
```

Behavior:

1. Load each recipe from Recipe Service.
2. Load pantry state once from Pantry Service.
3. Load ingredient metadata and conversions from Ingredient Dictionary as needed.
4. Aggregate non-optional recipe ingredients.
5. Normalize to the ingredient `default_unit` when a conversion path exists.
6. Subtract pantry quantities in the same normalized unit bucket.
7. Persist the generated list and only the items still needing purchase.

### GET /shopping-list/:id

Returns the saved list and saved items in the same shape produced by `POST /shopping-list`.

## Data Models

```text
shopping_lists
  id              UUID PK
  recipe_ids      UUID[]
  meal_plan_id    UUID NULLABLE
  created_at      TIMESTAMPTZ

shopping_list_items
  id                    UUID PK
  shopping_list_id      UUID FK
  ingredient_id         UUID
  name                  TEXT
  category              TEXT
  quantity_needed       FLOAT8
  quantity_in_pantry    FLOAT8
  quantity_to_buy       FLOAT8
  unit                  TEXT
  created_at            TIMESTAMPTZ
```

`internal/db/queries/shopping_lists.sql` is the source for generated `sqlc` persistence code.

## Implementation Notes

- Ingredients aggregate by canonical `ingredient_id`.
- Optional recipe ingredients are not included in the generated list.
- Unit normalization uses only ingredient-specific conversion rows from the Dictionary service.
- The conversion helper supports direct, inverse, and multi-step conversion paths.
- If no conversion path exists, the original unit is preserved and treated as a separate bucket.
- The service relies on upstream unit strings already being reasonably normalized. It does not maintain its own synonym table.
- Recipe ingredient names are not trusted from Recipe Service because the current upstream handler returns empty names in recipe detail responses; this service fetches names and categories from Ingredient Dictionary instead.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DB_URL` | required | PostgreSQL connection string for `shopping_list_db` |
| `RECIPE_URL` | required | Recipe Service base URL |
| `PANTRY_URL` | required | Pantry Service base URL |
| `DICTIONARY_URL` | required | Ingredient Dictionary base URL |
| `LOG_LEVEL` | `info` | Log level |

## Testing

- Unit tests cover conversion traversal, fallback behavior, and generation/diff logic.
- Handler tests cover `POST /shopping-list`, `GET /shopping-list/{id}`, and not-found handling.
- Integration-tagged coverage exercises the HTTP handlers with real service wiring and a PostgreSQL testcontainer when the environment permits container access.

## What To Avoid

- Do not query another service database directly.
- Do not add RabbitMQ.
- Do not guess at missing conversions.
- Do not invent new upstream APIs when existing service contracts are sufficient.
