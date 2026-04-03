# woodpantry-shopping-list

Shopping List Service for WoodPantry. Given a set of recipe IDs, this service fetches recipe ingredient requirements, subtracts current pantry stock, normalizes units when the Ingredient Dictionary provides conversion data, persists the generated list, and returns the saved result.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check |
| POST | `/shopping-list` | Generate and persist a shopping list from recipe IDs |
| GET | `/shopping-list/:id` | Fetch a previously generated shopping list |

## Request And Response Flow

### POST /shopping-list

Request body:

```json
{
  "recipe_ids": ["uuid", "uuid"],
  "meal_plan_id": "uuid-optional"
}
```

The service:

1. Calls `GET /recipes/{id}` on Recipe Service for each requested recipe.
2. Calls `GET /pantry` on Pantry Service once for current stock.
3. Calls `GET /ingredients/{id}` and `GET /ingredients/{id}/conversions` on Ingredient Dictionary for the ingredient metadata and unit conversions needed to normalize quantities.
4. Aggregates non-optional recipe ingredients across recipes.
5. Converts recipe and pantry quantities into the ingredient default unit when a conversion path exists.
6. Subtracts pantry stock from required quantities.
7. Persists the generated list and the items that still need to be purchased.

Response body:

```json
{
  "id": "uuid",
  "recipe_ids": ["uuid", "uuid"],
  "meal_plan_id": "uuid-optional",
  "created_at": "2026-04-03T15:00:00Z",
  "items": [
    {
      "id": "uuid",
      "ingredient_id": "uuid",
      "name": "flour",
      "category": "baking",
      "quantity_needed": 1.5,
      "quantity_in_pantry": 0.25,
      "quantity_to_buy": 1.25,
      "unit": "cup",
      "created_at": "2026-04-03T15:00:00Z"
    }
  ]
}
```

### GET /shopping-list/:id

Returns the persisted shopping list and its saved items in the same response shape as `POST /shopping-list`.

## Upstream Contracts Consumed

- Recipe Service: `GET /recipes/{id}`
- Pantry Service: `GET /pantry`
- Ingredient Dictionary: `GET /ingredients/{id}`
- Ingredient Dictionary: `GET /ingredients/{id}/conversions`

The service intentionally does not invent any new upstream APIs. It uses the currently implemented response shapes from those services, including the Ingredient Dictionary's capitalized JSON fields on `GET /ingredients/{id}`.

## Generation Rules

- Ingredients aggregate by canonical `ingredient_id`.
- Optional recipe ingredients are skipped.
- The Ingredient Dictionary `default_unit` is used as the normalization target when the service can find a conversion path.
- Conversion paths are resolved from the Dictionary conversion table and can use direct, inverse, or multi-step paths.
- If no conversion path exists, the original unit is preserved and aggregation continues within that unit only.
- Pantry stock is subtracted only when it normalizes into the same unit bucket as the recipe requirement.
- Items whose `quantity_to_buy` is zero or negative are not persisted as shopping-list items.

## Database

The service persists generated lists into:

```text
shopping_lists
  id
  recipe_ids
  meal_plan_id
  created_at

shopping_list_items
  id
  shopping_list_id
  ingredient_id
  name
  category
  quantity_needed
  quantity_in_pantry
  quantity_to_buy
  unit
  created_at
```

`sqlc` is generated from `internal/db/queries/shopping_lists.sql`.

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DB_URL` | required | PostgreSQL `shopping_list_db` connection string |
| `RECIPE_URL` | required | Recipe Service base URL |
| `PANTRY_URL` | required | Pantry Service base URL |
| `DICTIONARY_URL` | required | Ingredient Dictionary base URL |
| `LOG_LEVEL` | `info` | Log level |

## Development

```bash
go test ./...
go test ./... -tags=integration
go run ./cmd/shopping-list
sqlc generate -f internal/db/sqlc.yaml
```

The integration-tagged suite uses `testcontainers-go` for PostgreSQL and skips cleanly when Docker or Podman is unavailable in the environment.

## CI

GitHub Actions now matches the pattern used by the other Go services:

- pull request CI runs blocking lint, advisory lint, Docker build validation, unit tests, and integration-tagged tests
- push to `main` runs tests, builds the container image, and pushes `ghcr.io/<owner>/woodpantry-shopping-list`

## Current Conversion Limits

- Unit normalization only uses ingredient-specific conversions returned by the Dictionary service.
- No global synonym map exists in this service, so units like `cups` vs `cup` or `oz` vs `ounce` depend on upstream data already being normalized consistently.
- If the same ingredient appears in incompatible units and the Dictionary has no conversion path between them, the service keeps separate unit buckets instead of guessing.
