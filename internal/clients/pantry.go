package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/service"
)

type PantryClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewPantryClient(baseURL string, httpClient *http.Client) *PantryClient {
	return &PantryClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

type pantryListResponse struct {
	Items []struct {
		IngredientID string  `json:"ingredient_id"`
		Quantity     float64 `json:"quantity"`
		Unit         string  `json:"unit"`
	} `json:"items"`
}

func (c *PantryClient) ListPantry(ctx context.Context) ([]service.PantryItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/pantry", nil)
	if err != nil {
		return nil, fmt.Errorf("pantry list: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pantry list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pantry list: unexpected status %d", resp.StatusCode)
	}

	var decoded pantryListResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("pantry list decode: %w", err)
	}

	items := make([]service.PantryItem, 0, len(decoded.Items))
	for _, item := range decoded.Items {
		ingredientID, err := uuid.Parse(item.IngredientID)
		if err != nil {
			return nil, fmt.Errorf("pantry list decode ingredient_id: %w", err)
		}

		items = append(items, service.PantryItem{
			IngredientID: ingredientID,
			Quantity:     item.Quantity,
			Unit:         item.Unit,
		})
	}

	return items, nil
}
