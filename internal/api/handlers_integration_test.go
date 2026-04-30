//go:build integration

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/clients"
	"github.com/mwhite7112/woodpantry-shopping-list/internal/db"
	"github.com/mwhite7112/woodpantry-shopping-list/internal/service"
	"github.com/mwhite7112/woodpantry-shopping-list/internal/testutil"
)

func TestIntegration_CreateAndGetShoppingList(t *testing.T) {
	flourID := uuid.New()
	recipeOneID := uuid.New()
	recipeTwoID := uuid.New()

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/recipes/" + recipeOneID.String():
				return jsonResponse(t, http.StatusOK, map[string]any{
					"id": recipeOneID,
					"ingredients": []map[string]any{
						{"ingredient_id": flourID, "quantity": 1.0, "unit": "cup", "is_optional": false},
					},
				}), nil
			case "/recipes/" + recipeTwoID.String():
				return jsonResponse(t, http.StatusOK, map[string]any{
					"id": recipeTwoID,
					"ingredients": []map[string]any{
						{"ingredient_id": flourID, "quantity": 8.0, "unit": "tbsp", "is_optional": false},
					},
				}), nil
			case "/pantry":
				return jsonResponse(t, http.StatusOK, map[string]any{
					"items": []map[string]any{
						{"ingredient_id": flourID.String(), "quantity": 4.0, "unit": "tbsp"},
					},
				}), nil
			case "/ingredients/" + flourID.String():
				return jsonResponse(t, http.StatusOK, map[string]any{
					"ID":   flourID,
					"Name": "flour",
					"Category": map[string]any{
						"String": "baking",
						"Valid":  true,
					},
					"DefaultUnit": map[string]any{
						"String": "cup",
						"Valid":  true,
					},
				}), nil
			case "/ingredients/" + flourID.String() + "/conversions":
				return jsonResponse(t, http.StatusOK, []map[string]any{
					{"from_unit": "cup", "to_unit": "tbsp", "factor": 16.0},
				}), nil
			default:
				return jsonResponse(t, http.StatusNotFound, map[string]string{"error": "not found"}), nil
			}
		}),
	}

	sqlDB := testutil.SetupDB(t)
	router := NewRouter(service.New(
		sqlDB,
		db.New(sqlDB),
		clients.NewRecipeClient("http://upstream.test", httpClient),
		clients.NewPantryClient("http://upstream.test", httpClient),
		clients.NewDictionaryClient("http://upstream.test", httpClient),
	))

	req := httptest.NewRequest(
		http.MethodPost,
		"/shopping-list",
		strings.NewReader(`{"recipe_ids":["`+recipeOneID.String()+`","`+recipeTwoID.String()+`"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var created shoppingListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Len(t, created.Items, 1)
	assert.InDelta(t, 1.5, created.Items[0].QuantityNeeded, 0.0001)
	assert.InDelta(t, 0.25, created.Items[0].QuantityInPantry, 0.0001)
	assert.InDelta(t, 1.25, created.Items[0].QuantityToBuy, 0.0001)
	require.Len(t, created.Groups, 1)
	assert.Equal(t, "baking", created.Groups[0].Category)
	require.Len(t, created.Groups[0].Items, 1)
	assert.Equal(t, created.Items[0].ID, created.Groups[0].Items[0].ID)

	getReq := httptest.NewRequest(http.MethodGet, "/shopping-list/"+created.ID.String(), nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)

	require.Equal(t, http.StatusOK, getRec.Code)

	var loaded shoppingListResponse
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &loaded))
	assert.Equal(t, created.ID, loaded.ID)
	require.Len(t, loaded.Items, 1)
	assert.Equal(t, "cup", loaded.Items[0].Unit)
	require.Len(t, loaded.Groups, 1)
	assert.Equal(t, "baking", loaded.Groups[0].Category)
	require.Len(t, loaded.Groups[0].Items, 1)
	assert.Equal(t, loaded.Items[0].ID, loaded.Groups[0].Items[0].ID)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(t *testing.T, status int, value any) *http.Response {
	t.Helper()

	var body bytes.Buffer
	require.NoError(t, json.NewEncoder(&body).Encode(value))

	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body.Bytes())),
	}
}
