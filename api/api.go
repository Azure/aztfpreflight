package api

import (
	"context"
	"github.com/ms-henglu/aztfpreflight/utils"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"

	armruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type Client struct {
	host string
	pl   runtime.Pipeline
}

var c *Client

func NewClient() (*Client, error) {
	ep := cloud.AzurePublic.Services[cloud.ResourceManager].Endpoint
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
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
