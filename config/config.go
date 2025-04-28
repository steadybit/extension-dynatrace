/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-dynatrace/types"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Specification is the configuration specification for the extension. Configuration values can be applied
// through environment variables. Learn more through the documentation of the envconfig package.
// https://github.com/kelseyhightower/envconfig
type Specification struct {
	// The Dynatrace API Base Url, like 'https://{your-environment-id}.live.dynatrace.com/api' or 'https://{your-domain}/e/{your-environment-id}/api'
	ApiBaseUrl string `json:"apiBaseUrl" split_words:"true" required:"true"`
	// The Dynatrace UI Base Url, like 'https://{your-environment-id}.apps.dynatrace.com/ui'
	UiBaseUrl string `json:"uiBaseUrl" split_words:"true" required:"true"`
	// The Dynatrace API Token
	ApiToken string `json:"apiToken" split_words:"true" required:"true"`
}

var (
	Config Specification
)

func ParseConfiguration() {
	err := envconfig.Process("steadybit_extension", &Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}
}

func ValidateConfiguration() {
	// You may optionally validate the configuration here.
}

func (s *Specification) PostEvent(_ context.Context, event types.EventIngest) (*types.EventIngestResults, *http.Response, error) {
	b, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to marshal event")
		return nil, nil, err
	}

	responseBody, response, err := s.do(fmt.Sprintf("%s/v2/events/ingest", s.ApiBaseUrl), "POST", b)
	if err != nil {
		return nil, response, err
	}

	var result types.EventIngestResults
	if responseBody != nil {
		err = json.Unmarshal(responseBody, &result)
		if err != nil {
			log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed to parse response body")
			return nil, response, err
		}
	}
	return &result, response, err
}

func (s *Specification) GetEntities(_ context.Context, entitySelector string) (*types.EntitiesList, *http.Response, error) {
	responseBody, response, err := s.do(fmt.Sprintf("%s/v2/entities?entitySelector=%s", s.ApiBaseUrl, url.QueryEscape(entitySelector)), "GET", nil)
	if err != nil {
		return nil, response, err
	}

	var result types.EntitiesList
	if responseBody != nil {
		err = json.Unmarshal(responseBody, &result)
		if err != nil {
			log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed to parse body")
			return nil, response, err
		}
	}

	return &result, response, err
}

func (s *Specification) CreateMaintenanceWindow(_ context.Context, maintenanceWindow types.CreateMaintenanceWindowRequest) (*string, *http.Response, error) {
	objects := []types.CreateMaintenanceWindowRequest{maintenanceWindow}

	b, err := json.Marshal(objects)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to marshal request")
		return nil, nil, err
	}

	responseBody, response, err := s.do(fmt.Sprintf("%s/v2/settings/objects", s.ApiBaseUrl), "POST", b)
	if err != nil {
		return nil, response, err
	}

	if response.StatusCode != 200 {
		log.Error().Int("code", response.StatusCode).Err(err).Msgf("Unexpected response %+v", string(responseBody))
		return nil, response, fmt.Errorf("unexpected response code %d: %+v", response.StatusCode, string(responseBody))
	}

	var result []types.CreateMaintenanceWindowResponse
	if responseBody != nil {
		err = json.Unmarshal(responseBody, &result)
		if err != nil {
			log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed to parse response body")
			return nil, response, err
		}
	}

	if len(result) == 1 && result[0].Code == 200 {
		return &result[0].ObjectId, response, err
	} else {
		log.Error().Err(err).Msgf("Unexpected response %+v", result)
		return nil, response, errors.New("unexpected response")
	}
}

func (s *Specification) DeleteMaintenanceWindow(_ context.Context, maintenanceWindowId string) (*http.Response, error) {
	_, response, err := s.do(fmt.Sprintf("%s/v2/settings/objects/%s", s.ApiBaseUrl, maintenanceWindowId), "DELETE", nil)
	return response, err
}

func (s *Specification) GetProblems(_ context.Context, from time.Time, entitySelector *string) ([]types.Problem, *http.Response, error) {
	url := fmt.Sprintf("%s/v2/problems?problemSelector=status(\"OPEN\")&pageSize=500", s.ApiBaseUrl)
	if entitySelector != nil {
		url = fmt.Sprintf("%s&entitySelector=%s", url, *entitySelector)
	}

	responseBody, response, err := s.do(url, "GET", nil)
	if err != nil {
		return nil, response, err
	}

	if response.StatusCode != 200 {
		log.Error().Int("code", response.StatusCode).Err(err).Msgf("Unexpected response %+v", string(responseBody))
		return nil, response, fmt.Errorf("unexpected response code %d: %+v", response.StatusCode, string(responseBody))
	}

	var result types.GetProblemsResponse
	if responseBody != nil {
		err = json.Unmarshal(responseBody, &result)
		if err != nil {
			log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed to parse body")
			return nil, response, err
		}
	}

	return result.Problems, response, err
}

func (s *Specification) do(url string, method string, body []byte) ([]byte, *http.Response, error) {
	log.Debug().Str("url", url).Str("method", method).Msg("Requesting Dynatrace API")
	if body != nil {
		log.Debug().Int("len", len(body)).Str("body", string(body)).Msg("Request body")
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	request, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create request")
		return nil, nil, err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("Authorization", fmt.Sprintf("Api-Token %s", s.ApiToken))

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to execute request")
		return nil, response, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to close response body")
		}
	}(response.Body)

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to read body")
		return nil, response, err
	}

	return responseBody, response, err
}
