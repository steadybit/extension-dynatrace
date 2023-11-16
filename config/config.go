/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-dynatrace/types"
	"io"
	"net/http"
)

// Specification is the configuration specification for the extension. Configuration values can be applied
// through environment variables. Learn more through the documentation of the envconfig package.
// https://github.com/kelseyhightower/envconfig
type Specification struct {
	// The Dynatrace API Base Url, like 'https://{your-environment-id}.live.dynatrace.com/api' or 'https://{your-domain}/e/{your-environment-id}/api'
	ApiBaseUrl string `json:"apiBaseUrl" split_words:"true" required:"true"`
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

func (s *Specification) PostEvent(ctx context.Context, event types.EventIngest) (*types.EventIngestResults, *http.Response, error) {
	url := fmt.Sprintf("%s/v2/events/ingest", s.ApiBaseUrl)
	b, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to marshal event")
		return nil, nil, err
	}

	log.Debug().Str("body", string(b)).Msgf("Posting event to %s", url)

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
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

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to read body")
		return nil, response, err
	}

	var result types.EventIngestResults
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Error().Err(err).Str("body", string(body)).Msgf("Failed to parse body")
		return nil, response, err
	}

	return &result, response, err
}
