package interceptor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	InterceptedErrorCode = "InterceptedError"
)

var cache = make(map[string]string)

func HandleRequest(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if req.Method == "POST" && strings.Contains(req.URL.Path, "checkNameAvailability") {
		response := make(map[string]bool)
		response["nameAvailable"] = true
		data, _ := json.Marshal(response)

		return &http.Response{
			StatusCode: 200,
			Header: map[string][]string{
				"Content-Type":   {"application/json"},
				"Content-Length": {fmt.Sprintf("%d", len(data))},
			},
			ContentLength: int64(len(data)),
			Body:          io.NopCloser(bytes.NewReader(data)),
			Request:       req,
		}, nil
	}

	if req.Method == "GET" || req.Method == "HEAD" {
		if existing := cache[req.URL.String()]; existing != "" {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte(existing))),
				Header: map[string][]string{
					"Content-Type":   {"application/json"},
					"Content-Length": {fmt.Sprintf("%d", len([]byte(existing)))},
				},
				ContentLength: int64(len([]byte(existing))),
				Request:       req,
			}, nil
		}

		return &http.Response{
			StatusCode: 404,
			Body:       http.NoBody,
			Header: map[string][]string{
				"Content-Type": {"application/json"},
			},
			Request: req,
		}, nil
	}

	if req.Method == "PUT" || req.Method == "PATCH" || req.Method == "POST" {
		requestBody := requestBodyString(req)
		model := ServiceError{
			Code:    InterceptedErrorCode,
			Message: InterceptedErrorCode,
			InnerError: map[string]interface{}{
				"url":  req.URL.String(),
				"body": requestBody,
			},
		}
		data, _ := json.Marshal(model)

		cache[req.URL.String()] = requestBody

		return &http.Response{
			StatusCode: 400,
			Header: map[string][]string{
				"Content-Type":   {"application/json"},
				"Content-Length": {fmt.Sprintf("%d", len(data))},
			},
			ContentLength: int64(len(data)),
			Body:          io.NopCloser(bytes.NewReader(data)),
			Request:       req,
		}, nil
	}

	return &http.Response{
		StatusCode: 400,
		Header: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body:    http.NoBody,
		Request: req,
	}, nil
}

func requestBodyString(req *http.Request) string {
	if req == nil || req.Body == nil {
		return ""
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		body = []byte(err.Error())
	} else {
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	return string(body)
}

type ServiceError struct {
	Code           string                   `json:"code"`
	Message        string                   `json:"message"`
	Target         *string                  `json:"target"`
	Details        []map[string]interface{} `json:"details"`
	InnerError     map[string]interface{}   `json:"innererror"`
	AdditionalInfo []map[string]interface{} `json:"additionalInfo"`
}
