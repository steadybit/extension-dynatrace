// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

/*
 * Copyright 2022 steadybit GmbH. All rights reserved.
 */

package extevents

import (
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/steadybit/event-kit/go/event_kit_api"
	"github.com/steadybit/extension-dynatrace/types"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hash/fnv"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func Test_addBaseProperties(t *testing.T) {
	type args struct {
		event event_kit_api.EventRequestBody
	}

	eventTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Successfully add base properties",
			args: args{
				event: event_kit_api.EventRequestBody{
					Environment: extutil.Ptr(event_kit_api.Environment{
						Id:   "test",
						Name: "gateway",
					}),
					EventName: "experiment.started",
					EventTime: eventTime,
					Id:        uuid.MustParse("ccf6a26e-588f-446e-8eaa-d16b086e150e"),
					Principal: event_kit_api.UserPrincipal{
						Email:         extutil.Ptr("email"),
						Name:          "Peter",
						Username:      "Pan",
						PrincipalType: string(event_kit_api.User),
					},
					Team: extutil.Ptr(event_kit_api.Team{
						Id:   "test",
						Key:  "test",
						Name: "gateway",
					}),
					Tenant: event_kit_api.Tenant{
						Key:  "key",
						Name: "name",
					},
				},
			},
			want: map[string]string{
				"steadybit.environment.name":   "gateway",
				"steadybit.principal.name":     "Peter",
				"steadybit.principal.type":     "user",
				"steadybit.principal.username": "Pan",
				"steadybit.team.key":           "test",
				"steadybit.team.name":          "gateway",
			},
		},
		{
			name: "Successfully add base properties without Principal",
			args: args{
				event: event_kit_api.EventRequestBody{
					Environment: extutil.Ptr(event_kit_api.Environment{
						Id:   "test",
						Name: "gateway",
					}),
					EventName: "experiment.started",
					EventTime: eventTime,
					Id:        uuid.MustParse("ccf6a26e-588f-446e-8eaa-d16b086e150e"),
					Principal: event_kit_api.AccessTokenPrincipal{
						Name:          "MyFancyToken",
						PrincipalType: string(event_kit_api.AccessToken),
					},
					Team: extutil.Ptr(event_kit_api.Team{
						Id:   "test",
						Key:  "test",
						Name: "gateway",
					}),
					Tenant: event_kit_api.Tenant{
						Key:  "key",
						Name: "name",
					},
				},
			},
			want: map[string]string{
				"steadybit.environment.name": "gateway",
				"steadybit.principal.name":   "MyFancyToken",
				"steadybit.principal.type":   "access_token",
				"steadybit.team.key":         "test",
				"steadybit.team.name":        "gateway",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := make(map[string]string)
			addBaseProperties(props, &tt.args.event)
			assert.Equalf(t, tt.want, props, "addBaseProperties(%v)", tt.args.event)
		})
	}
}

func Test_addExperimentExecutionProperties(t *testing.T) {
	type args struct {
		event event_kit_api.EventRequestBody
	}

	eventTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	endedTime := time.Date(2021, 1, 1, 0, 2, 0, 0, time.UTC)
	startedTime := time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Successfully add execution properties",
			args: args{
				event: event_kit_api.EventRequestBody{
					ExperimentExecution: extutil.Ptr(event_kit_api.ExperimentExecution{
						EndedTime:     extutil.Ptr(endedTime),
						ExecutionId:   42,
						ExperimentKey: "ExperimentKey",
						Reason:        extutil.Ptr("Reason"),
						ReasonDetails: extutil.Ptr("ReasonDetails"),
						Hypothesis:    "Hypothesis",
						Name:          "Name",
						PreparedTime:  eventTime,
						StartedTime:   startedTime,
						State:         event_kit_api.ExperimentExecutionStateCreated,
					}),
				},
			},
			want: map[string]string{
				"steadybit.execution.id":          "42",
				"steadybit.execution.state":       "created",
				"steadybit.experiment.hypothesis": "Hypothesis",
				"steadybit.experiment.key":        "ExperimentKey",
				"steadybit.experiment.name":       "Name",
			},
		},
		{
			name: "Successfully add execution properties without hypothesis",
			args: args{
				event: event_kit_api.EventRequestBody{
					ExperimentExecution: extutil.Ptr(event_kit_api.ExperimentExecution{
						EndedTime:     extutil.Ptr(endedTime),
						ExecutionId:   42,
						ExperimentKey: "ExperimentKey",
						Reason:        extutil.Ptr("Reason"),
						ReasonDetails: extutil.Ptr("ReasonDetails"),
						Hypothesis:    "",
						Name:          "Name",
						PreparedTime:  eventTime,
						StartedTime:   startedTime,
						State:         event_kit_api.ExperimentExecutionStateCreated,
					}),
				},
			},
			want: map[string]string{
				"steadybit.execution.id":    "42",
				"steadybit.execution.state": "created",
				"steadybit.experiment.key":  "ExperimentKey",
				"steadybit.experiment.name": "Name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := make(map[string]string)
			addExperimentExecutionProperties(props, tt.args.event.ExperimentExecution)
			assert.Equalf(t, tt.want, props, "addExperimentExecutionProperties(%v)", tt.args.event)
		})
	}
}

func Test_addStepExecutionProperties(t *testing.T) {
	type args struct {
		w             http.ResponseWriter
		stepExecution event_kit_api.ExperimentStepExecution
	}

	endedTime := time.Date(2021, 1, 1, 0, 2, 0, 0, time.UTC)
	startedTime := time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Successfully add properties for started attack",
			args: args{
				stepExecution: event_kit_api.ExperimentStepExecution{
					Id:          uuid.UUID{},
					Type:        event_kit_api.Action,
					ActionId:    extutil.Ptr("com.steadybit.action.example"),
					ActionName:  extutil.Ptr("example-action"),
					ActionKind:  extutil.Ptr(event_kit_api.Attack),
					CustomLabel: extutil.Ptr("My very own label"),
					State:       event_kit_api.ExperimentStepExecutionStateFailed,
					EndedTime:   extutil.Ptr(endedTime),
					StartedTime: extutil.Ptr(startedTime),
				},
			},
			want: map[string]string{
				"steadybit.step.action.custom_label": "My very own label",
				"steadybit.step.action.id":           "com.steadybit.action.example",
				"steadybit.step.action.name":         "example-action",
			},
		},
		{
			name: "Successfully add properties for not yet started attack",
			args: args{
				stepExecution: event_kit_api.ExperimentStepExecution{
					Id:         uuid.UUID{},
					Type:       event_kit_api.Action,
					ActionId:   extutil.Ptr("com.steadybit.action.example"),
					ActionKind: extutil.Ptr(event_kit_api.Attack),
					State:      event_kit_api.ExperimentStepExecutionStateCompleted,
				},
			},
			want: map[string]string{
				"steadybit.step.action.id": "com.steadybit.action.example",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := make(map[string]string)
			addStepExecutionProperties(props, &tt.args.stepExecution)
			assert.Equalf(t, tt.want, props, "addStepExecutionProperties(%v)", tt.args.stepExecution)
		})
	}
}

func hash(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return strconv.FormatUint(uint64(h.Sum32()), 10)
}

func Test_addTargetExecutionProperties(t *testing.T) {
	mockLoader := ttlcache.LoaderFunc[string, string](
		func(c *ttlcache.Cache[string, string], key string) *ttlcache.Item[string, string] {
			return c.Set(key, hash(key), ttlcache.DefaultTTL)
		},
	)
	entityCache = ttlcache.New[string, string](
		ttlcache.WithLoader[string, string](mockLoader),
		ttlcache.WithTTL[string, string](30*time.Minute),
	)

	type args struct {
		w      http.ResponseWriter
		target event_kit_api.ExperimentStepTargetExecution
	}

	endedTime := time.Date(2021, 1, 1, 0, 2, 0, 0, time.UTC)
	startedTime := time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)
	id := uuid.New()
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Successfully get properties for container targets",
			args: args{
				target: event_kit_api.ExperimentStepTargetExecution{
					ExecutionId:   42,
					ExperimentKey: "ExperimentKey",
					Id:            id,
					State:         "completed",
					AgentHostname: "Agent-1",
					TargetAttributes: map[string][]string{
						"k8s.container.name":                       {"example-c1"},
						"k8s.pod.label.tags.datadoghq.com/service": {"example-service"},
						"container.host":                           {"host-123"},
						"k8s.namespace":                            {"namespace"},
						"k8s.deployment":                           {"example"},
						"k8s.pod.name":                             {"example-4711-123"},
						"k8s.cluster-name":                         {"dev-cluster"},
						"aws.zone":                                 {"eu-central-1a"},
						"aws.region":                               {"eu-central-1"},
						"aws.account":                              {"123456789"},
					},
					TargetName:  "Container",
					TargetType:  "com.steadybit.extension_container.container",
					StartedTime: &startedTime,
					EndedTime:   &endedTime,
				},
			},
			want: map[string]string{
				"dt.entity.cloud_application":           "2315379620",
				"dt.entity.cloud_application_instance":  "438567049",
				"dt.entity.cloud_application_namespace": "61947927",
				"dt.entity.kubernetes_cluster":          "216475693",
				"dt.entity.kubernetes_node":             "1237239306",
				"steadybit.execution.id":                "42",
				"steadybit.execution.target.state":      "completed",
				"steadybit.experiment.key":              "ExperimentKey",
			},
		},
		{
			name: "Ignore multiple values",
			args: args{
				target: event_kit_api.ExperimentStepTargetExecution{
					ExecutionId:   42,
					ExperimentKey: "ExperimentKey",
					Id:            id,
					State:         "completed",
					AgentHostname: "Agent-1",
					TargetAttributes: map[string][]string{
						"host.hostname": {"Host-1"},
						"k8s.namespace": {"namespace-1", "namespace-2", "namespace-3"},
					},
					TargetName:  "Host",
					TargetType:  "com.steadybit.extension_host.host",
					StartedTime: &startedTime,
					EndedTime:   &endedTime,
				},
			},
			want: map[string]string{
				"steadybit.execution.id":           "42",
				"steadybit.execution.target.state": "completed",
				"steadybit.experiment.key":         "ExperimentKey",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := make(map[string]string)
			addTargetExecutionProperties(props, &tt.args.target)
			assert.Equalf(t, tt.want, props, "addTargetExecutionProperties(%v)", tt.args.target)
		})
	}
}

func Test_onExperimentStepStarted(t *testing.T) {
	eventTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	startedTime := time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)
	endedTime := time.Date(2021, 1, 1, 0, 7, 0, 0, time.UTC)
	stepId := uuid.MustParse("ccf6a26e-588f-446e-8eaa-d16b086e150e")

	type args struct {
		stepEvent   event_kit_api.EventRequestBody
		targetEvent event_kit_api.EventRequestBody
	}
	tests := []struct {
		name string
		args args
		want *types.EventIngest
	}{
		{
			name: "should emit event for experiment target started",
			args: args{
				stepEvent: event_kit_api.EventRequestBody{
					Environment: extutil.Ptr(event_kit_api.Environment{
						Id:   "test",
						Name: "gateway",
					}),
					EventName: "experiment.step.started",
					EventTime: eventTime,
					Id:        stepId,
					ExperimentStepExecution: extutil.Ptr(event_kit_api.ExperimentStepExecution{
						ExecutionId:   42,
						ExperimentKey: "ExperimentKey",
						Id:            stepId,
						ActionId:      extutil.Ptr("some_action_id"),
						ActionName:    extutil.Ptr("started step"),
						CustomLabel:   extutil.Ptr("custom label"),
						ActionKind:    extutil.Ptr(event_kit_api.Attack),
					}),
					Tenant: event_kit_api.Tenant{
						Key:  "key",
						Name: "name",
					},
				},
				targetEvent: event_kit_api.EventRequestBody{
					Environment: extutil.Ptr(event_kit_api.Environment{
						Id:   "test",
						Name: "gateway",
					}),
					EventName: "experiment.step.target.started",
					EventTime: eventTime,
					Id:        stepId,
					ExperimentStepTargetExecution: extutil.Ptr(event_kit_api.ExperimentStepTargetExecution{
						ExecutionId:     42,
						ExperimentKey:   "ExperimentKey",
						StepExecutionId: stepId,
						State:           "completed",
						TargetType:      "type",
						TargetName:      "test",
						StartedTime:     &startedTime,
						EndedTime:       &endedTime,
					}),
					Tenant: event_kit_api.Tenant{
						Key:  "key",
						Name: "name",
					},
				},
			},
			want: &types.EventIngest{
				Properties: map[string]string{
					"steadybit.environment.name":         "gateway",
					"steadybit.execution.id":             "42",
					"steadybit.execution.target.state":   "completed",
					"steadybit.experiment.key":           "ExperimentKey",
					"steadybit.step.action.custom_label": "custom label",
					"steadybit.step.action.name":         "started step",
				},
				StartTime: extutil.Ptr(int64(1609459260000)),
				EndTime:   extutil.Ptr(int64(1609459260000)),
				EventType: "CUSTOM_INFO",
				Title:     "Steadybit experiment 'ExperimentKey / 42' - Attack 'custom label' started - Target 'test'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := onExperimentStepStarted(&tt.args.stepEvent)
			require.NoError(t, err)
			got, err := onExperimentTargetStarted(&tt.args.targetEvent)
			require.NoError(t, err)
			assert.Equalf(t, tt.want.Properties, got.Properties, "onExperimentTargetStarted - Properties different")
			assert.Equalf(t, tt.want, got, "onExperimentTargetStarted - Something else ist different, take a very close look ;-)")
		})
	}
}
