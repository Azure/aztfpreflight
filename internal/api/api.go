package api

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Azure/aztfpreflight/internal/utils"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	armruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/sirupsen/logrus"
)

type Client struct {
	host string
	pl   runtime.Pipeline
}

var c *Client

// envTokenCredential is a simple TokenCredential implementation that returns
// a static token read from the environment. This allows callers to supply a
// bearer token via an environment variable (e.g. for CI or debugging).
//
// Note: The supplied token must be an OAuth2 access token appropriate for
// Azure Resource Manager (for example obtained with the scope
// `https://management.azure.com/.default`). The token is used as-is and will
// be added to the Authorization header by the SDK. Because externally
// supplied tokens may not include expiry metadata, this credential sets a
// conservative expiry of 1 hour. Use this primarily for short-lived CI or
// debugging scenarios. For long-running processes prefer a refreshable
// credential such as DefaultAzureCredential.
type envTokenCredential struct {
	token string
}

func (e *envTokenCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	// Return the env token as the access token. The SDK will include this in
	// the Authorization header. Expiry is set to now + 1h since we cannot
	// infer it from the supplied token.
	return azcore.AccessToken{Token: e.token, ExpiresOn: time.Now().Add(1 * time.Hour)}, nil
}

func NewClient() (*Client, error) {
	ep := cloud.AzurePublic.Services[cloud.ResourceManager].Endpoint

	// If AZURE_ACCESS_TOKEN is set then use it as a static token credential.
	// Otherwise, use the Azure SDK's DefaultAzureCredential. The static token
	// will be returned as-is and used for the Authorization header; this is
	// intended for CI and short-lived tokens.
	var cred azcore.TokenCredential
	if t := os.Getenv("AZURE_ACCESS_TOKEN"); t != "" {
		// Static env-supplied token takes precedence.
		cred = &envTokenCredential{token: t}
	} else {
		var err error
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}
	}

	pl, err := armruntime.NewPipeline("aztfpreflight", "dev", cred, runtime.PipelineOptions{}, nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		host: ep,
		pl:   pl,
	}, nil
}

func DefaultSharedClient() (*Client, error) {
	if c != nil {
		return c, nil
	}
	client, err := NewClient()
	if err != nil {
		return nil, err
	}

	c = client
	return c, nil
}

func Execute[ResponseT interface{}](ctx context.Context, client *Client, method string, url string, apiVersion string, body interface{}) (*ResponseT, error) {
	logrus.Debugf("Executing request %s %s\nrequest body: %s", method, url, utils.ToJson(body))
	req, err := runtime.NewRequest(ctx, method, runtime.JoinPaths(client.host, url))
	if err != nil {
		return nil, err
	}
	reqQP := req.Raw().URL.Query()
	reqQP.Set("api-version", apiVersion)
	req.Raw().URL.RawQuery = reqQP.Encode()
	req.Raw().Header.Set("Accept", "application/json")
	err = runtime.MarshalAsJSON(req, body)
	if err != nil {
		return nil, err
	}

	resp, err := client.pl.Do(req)
	if err != nil {
		return nil, err
	}
	if !runtime.HasStatusCode(resp, http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent) {
		return nil, runtime.NewResponseError(resp)
	}
	logrus.Debugf("response status code: %d", resp.StatusCode)
	responseBody := new(ResponseT)
	contentType := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(contentType, "application/json"):
		if err := runtime.UnmarshalAsJSON(resp, &responseBody); err != nil {
			logrus.Errorf("failed to unmarshal response body: %s", err)
			return nil, err
		}
		logrus.Debugf("response body: %s", utils.ToJson(responseBody))
	default:
	}
	return responseBody, nil
}
