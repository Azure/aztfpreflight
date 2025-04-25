package types

import (
	"encoding/json"
	"regexp"

	"github.com/Azure/go-autorest/autorest/azure"
)

var errorParsers []ErrorParser

func init() {
	errorParsers = []ErrorParser{
		NewAutoRestErrorParser(),
		NewAutoRestPollerErrorParser(),
	}
}

func NewRequestModelsFromError(input string) []RequestModel {
	for _, parser := range errorParsers {
		models := parser.ParseError(input)
		if len(models) > 0 {
			return models
		}
	}
	return nil
}

type RequestModel struct {
	URL     string `json:"url"`
	Body    string `json:"body"`
	Address string `json:"address"`
	Failed  *FailedCase
}

type FailedCase struct {
	TestcasePath string
	Detail       string
}

type ErrorParser interface {
	ParseError(input string) []RequestModel
}

type AutoRestErrorParser struct {
	r *regexp.Regexp
}

func NewAutoRestErrorParser() ErrorParser {
	return AutoRestErrorParser{
		r: regexp.MustCompile(`unexpected status \d+ with response: (.+)`),
	}
}

func (p AutoRestErrorParser) ParseError(input string) []RequestModel {
	matches := p.r.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]RequestModel, 0)
	for _, match := range matches {
		var serverError azure.ServiceError
		err := json.Unmarshal([]byte(match[1]), &serverError)
		if err != nil {
			continue
		}

		if serverError.InnerError == nil {
			continue
		}

		url := serverError.InnerError["url"].(string)
		bodyJson := serverError.InnerError["body"].(string)
		model := RequestModel{
			URL:  url,
			Body: bodyJson,
		}
		result = append(result, model)
	}
	return result
}

var _ ErrorParser = AutoRestErrorParser{}

type AutoRestPollerErrorParser struct {
	r *regexp.Regexp
}

func NewAutoRestPollerErrorParser() ErrorParser {
	return AutoRestPollerErrorParser{
		r: regexp.MustCompile(`Code="InterceptedError" Message="InterceptedError" InnerError=(.+)`),
	}
}

func (p AutoRestPollerErrorParser) ParseError(input string) []RequestModel {
	matches := p.r.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]RequestModel, 0)
	for _, match := range matches {
		var innerError map[string]interface{}
		err := json.Unmarshal([]byte(match[1]), &innerError)
		if err != nil {
			continue
		}

		if len(innerError) == 0 {
			continue
		}
		url := ""
		bodyJson := ""
		if urlVal, ok := innerError["url"]; ok {
			url = urlVal.(string)
		}
		if url == "" {
			continue
		}
		if bodyVal, ok := innerError["body"]; ok {
			bodyJson = bodyVal.(string)
		}
		model := RequestModel{
			URL:  url,
			Body: bodyJson,
		}
		result = append(result, model)
	}
	return result
}
