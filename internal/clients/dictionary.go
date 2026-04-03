package clients

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/service"
)

type DictionaryClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewDictionaryClient(baseURL string, httpClient *http.Client) *DictionaryClient {
	return &DictionaryClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

type ingredientResponse struct {
	ID          uuid.UUID      `json:"ID"`
	Name        string         `json:"Name"`
	Category    sql.NullString `json:"Category"`
	DefaultUnit sql.NullString `json:"DefaultUnit"`
}

type conversionResponse struct {
	FromUnit string  `json:"from_unit"`
	ToUnit   string  `json:"to_unit"`
	Factor   float64 `json:"factor"`
}

func (c *DictionaryClient) GetIngredient(ctx context.Context, id uuid.UUID) (service.IngredientDetail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ingredients/"+id.String(), nil)
	if err != nil {
		return service.IngredientDetail{}, fmt.Errorf("dictionary get ingredient: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return service.IngredientDetail{}, fmt.Errorf("dictionary get ingredient: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return service.IngredientDetail{}, fmt.Errorf("dictionary get ingredient: unexpected status %d", resp.StatusCode)
	}

	var decoded ingredientResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return service.IngredientDetail{}, fmt.Errorf("dictionary get ingredient decode: %w", err)
	}

	return service.IngredientDetail{
		ID:          decoded.ID,
		Name:        decoded.Name,
		Category:    decoded.Category.String,
		DefaultUnit: decoded.DefaultUnit.String,
	}, nil
}

func (c *DictionaryClient) ListConversions(ctx context.Context, id uuid.UUID) ([]service.UnitConversion, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/ingredients/"+id.String()+"/conversions",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("dictionary list conversions: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dictionary list conversions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dictionary list conversions: unexpected status %d", resp.StatusCode)
	}

	var decoded []conversionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("dictionary list conversions decode: %w", err)
	}

	conversions := make([]service.UnitConversion, 0, len(decoded))
	for _, conversion := range decoded {
		conversions = append(conversions, service.UnitConversion{
			FromUnit: conversion.FromUnit,
			ToUnit:   conversion.ToUnit,
			Factor:   conversion.Factor,
		})
	}

	return conversions, nil
}
