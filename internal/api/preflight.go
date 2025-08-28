package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/Azure/aztfpreflight/internal/types"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/sirupsen/logrus"
)

type PreflightRequestModel struct {
	Provider  string                   `json:"provider"`
	Type      string                   `json:"type"`
	Location  string                   `json:"location"`
	Scope     string                   `json:"scope"`
	Resources []map[string]interface{} `json:"resources"`
}

type PreflightResponseModel struct {
	Properties PropertiesModel `json:"properties"`
}

type PropertiesModel struct {
	ValidatedResources []string `json:"validatedResources"`
}

func Preflight(ctx context.Context, model PreflightRequestModel) (interface{}, error) {
	client, err := DefaultSharedClient()
	if err != nil {
		return nil, err
	}
	resp, err := Execute[PreflightResponseModel](ctx, client, http.MethodPost, "/providers/Microsoft.Resources/validateResources", "2020-10-01", model)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func PreflightInBatch(ctx context.Context, requests []types.RequestModel, concurrency int) []error {
	preflightRequests := make([]PreflightRequestModel, 0, len(requests))
	preflightErrors := make([]error, 0)
	for _, req := range requests {
		preflightRequest, err := buildPreflightRequestBody(req)
		if err != nil {
			preflightErrors = append(preflightErrors, err)
			continue
		}
		preflightRequests = append(preflightRequests, preflightRequest)
	}

	// group the requests by provider, type, location, scope
	groupedRequests := make(map[string]*PreflightRequestModel)
	for _, r := range preflightRequests {
		key := preflightRequestKey(r)
		if existing, ok := groupedRequests[key]; ok {
			existing.Resources = append(existing.Resources, r.Resources...)
			groupedRequests[key] = existing
		} else {
			groupedRequests[key] = &r
		}
	}
	logrus.Debugf("Grouped %d requests into %d preflight requests", len(preflightRequests), len(groupedRequests))

	sem := make(chan struct{}, concurrency)
	var mu = &sync.Mutex{}
	var wg sync.WaitGroup
	for _, r := range groupedRequests {
		if r == nil {
			continue
		}
		r := r // capture loop variable
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := Preflight(ctx, *r); err != nil {
				mu.Lock()
				preflightErrors = append(preflightErrors, err)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return preflightErrors
}

func preflightRequestKey(r PreflightRequestModel) string {
	return strings.Join([]string{r.Provider, r.Type, r.Location, r.Scope}, "|")
}

func buildPreflightRequestBody(request types.RequestModel) (PreflightRequestModel, error) {
	parsedUrl, err := url.Parse(request.URL)
	if err != nil {
		return PreflightRequestModel{}, err
	}

	armId, err := arm.ParseResourceID(parsedUrl.Path)
	if err != nil {
		return PreflightRequestModel{}, err
	}

	var payloadMap map[string]interface{}
	if err := json.Unmarshal([]byte(request.Body), &payloadMap); err != nil {
		return PreflightRequestModel{}, err
	}

	location := ""
	if loc, ok := payloadMap["location"]; ok {
		location = loc.(string)
	}

	scopeId := armId.Parent
	for scopeId.Parent != nil && scopeId.ResourceType.String() != arm.SubscriptionResourceType.String() &&
		scopeId.ResourceType.String() != arm.ResourceGroupResourceType.String() &&
		scopeId.ResourceType.String() != arm.TenantResourceType.String() {
		scopeId = scopeId.Parent
	}

	payloadMap["apiVersion"] = parsedUrl.Query().Get("api-version")
	payloadMap["name"] = armId.Name
	preflightRequestModel := PreflightRequestModel{
		Provider: armId.ResourceType.Namespace,
		Type:     armId.ResourceType.Type,
		Location: normalizeLocation(location),
		Scope:    scopeId.String(),
		Resources: []map[string]interface{}{
			payloadMap,
		},
	}
	return preflightRequestModel, nil
}

func normalizeLocation(input string) string {
	return strings.ReplaceAll(strings.ToLower(input), " ", "")
}
