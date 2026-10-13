// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extevents

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/steadybit/event-kit/go/event_kit_api"
	"github.com/steadybit/extension-dynatrace/config"
	"github.com/steadybit/extension-dynatrace/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useEntityCache replaces the global entity cache with one backed by the given lookup table.
// Selectors not present in the table resolve to an empty entity id (= not found).
func useEntityCache(t *testing.T, entities map[string]string) {
	t.Helper()
	previous := entityCache
	entityCache = ttlcache.New(
		ttlcache.WithLoader(ttlcache.LoaderFunc[string, string](
			func(c *ttlcache.Cache[string, string], key string) *ttlcache.Item[string, string] {
				return c.Set(key, entities[key], ttlcache.DefaultTTL)
			},
		)),
	)
	t.Cleanup(func() { entityCache = previous })
}

func storeStepExecution(t *testing.T, step event_kit_api.ExperimentStepExecution) {
	t.Helper()
	stepExecutions.Store(step.Id, step)
	t.Cleanup(func() { stepExecutions.Delete(step.Id) })
}

func Test_getEntitySelector(t *testing.T) {
	tests := []struct {
		name       string
		targetType string
		attributes map[string][]string
		want       *string
	}{
		{
			name:       "kubernetes cluster",
			targetType: "com.steadybit.extension_kubernetes.kubernetes-cluster",
			attributes: map[string][]string{"k8s.cluster-name": {"dev"}},
			want:       new("type(KUBERNETES_CLUSTER),entityName.equals(dev)"),
		},
		{
			name:       "kubernetes deployment",
			targetType: "com.steadybit.extension_kubernetes.kubernetes-deployment",
			attributes: map[string][]string{"k8s.deployment": {"shop"}},
			want:       new("type(CLOUD_APPLICATION),entityName.equals(shop)"),
		},
		{
			name:       "kubernetes statefulset",
			targetType: "com.steadybit.extension_kubernetes.kubernetes-statefulset",
			attributes: map[string][]string{"k8s.statefulset": {"db"}},
			want:       new("type(CLOUD_APPLICATION),entityName.equals(db)"),
		},
		{
			name:       "kubernetes daemonset",
			targetType: "com.steadybit.extension_kubernetes.kubernetes-daemonset",
			attributes: map[string][]string{"k8s.daemonset": {"agent"}},
			want:       new("type(CLOUD_APPLICATION),entityName.equals(agent)"),
		},
		{
			name:       "kubernetes node",
			targetType: "com.steadybit.extension_kubernetes.kubernetes-node",
			attributes: map[string][]string{"k8s.node.name": {"node-1"}},
			want:       new("type(KUBERNETES_NODE),entityName.equals(node-1)"),
		},
		{
			name:       "kubernetes pod",
			targetType: "com.steadybit.extension_kubernetes.kubernetes-pod",
			attributes: map[string][]string{"k8s.pod.name": {"pod-1"}},
			want:       new("type(CLOUD_APPLICATION_INSTANCE),entityName.equals(pod-1)"),
		},
		{
			name:       "jvm application",
			targetType: "com.steadybit.extension_jvm.application",
			attributes: map[string][]string{"k8s.pod.name": {"pod-1"}},
			want:       new("type(CLOUD_APPLICATION_INSTANCE),entityName.equals(pod-1)"),
		},
		{
			name:       "container",
			targetType: "com.steadybit.extension_container.container",
			attributes: map[string][]string{"k8s.pod.name": {"pod-1"}, "k8s.container.name": {"app"}},
			want:       new("type(CONTAINER_GROUP_INSTANCE),entityName.equals(pod-1 app)"),
		},
		{
			name:       "container without pod name",
			targetType: "com.steadybit.extension_container.container",
			attributes: map[string][]string{"k8s.container.name": {"app"}},
			want:       nil,
		},
		{
			name:       "host in kubernetes cluster",
			targetType: "com.steadybit.extension_host.host",
			attributes: map[string][]string{"host.hostname": {"node-1"}, "k8s.cluster-name": {"dev"}},
			want:       new("type(KUBERNETES_NODE),entityName.equals(node-1)"),
		},
		{
			name:       "plain host",
			targetType: "com.steadybit.extension_host.host",
			attributes: map[string][]string{"host.hostname": {"host-1"}},
			want:       new("type(HOST),entityName.equals(host-1)"),
		},
		{
			name:       "multiple values are ignored",
			targetType: "com.steadybit.extension_kubernetes.kubernetes-deployment",
			attributes: map[string][]string{"k8s.deployment": {"a", "b"}},
			want:       nil,
		},
		{
			name:       "unknown target type",
			targetType: "com.example.unknown",
			attributes: map[string][]string{"host.hostname": {"host-1"}},
			want:       nil,
		},
		{
			name:       "entity not found in dynatrace",
			targetType: "com.steadybit.extension_host.host",
			attributes: map[string][]string{"host.hostname": {"unknown-host"}},
			want:       nil,
		},
	}

	known := map[string]string{}
	for _, tt := range tests {
		if tt.want != nil {
			known[*tt.want] = "ENTITY-" + tt.name
		}
	}
	useEntityCache(t, known)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEntitySelector(event_kit_api.ExperimentStepTargetExecution{
				TargetType:       tt.targetType,
				TargetAttributes: tt.attributes,
			})
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getActionName(t *testing.T) {
	assert.Equal(t, "action-id", getActionName(event_kit_api.ExperimentStepExecution{ActionId: new("action-id")}))
	assert.Equal(t, "Action Name", getActionName(event_kit_api.ExperimentStepExecution{ActionId: new("action-id"), ActionName: new("Action Name")}))
	assert.Equal(t, "Custom", getActionName(event_kit_api.ExperimentStepExecution{ActionId: new("action-id"), ActionName: new("Action Name"), CustomLabel: new("Custom")}))
}

func Test_getTargetName(t *testing.T) {
	assert.Equal(t, "target", getTargetName(event_kit_api.ExperimentStepTargetExecution{TargetName: "target"}))
	assert.Equal(t, "label", getTargetName(event_kit_api.ExperimentStepTargetExecution{
		TargetName:       "target",
		TargetAttributes: map[string][]string{"steadybit.label": {"label"}},
	}))
}

func Test_addBaseProperties_BatchPrincipalWithoutTeam(t *testing.T) {
	props := make(map[string]string)
	addBaseProperties(props, &event_kit_api.EventRequestBody{
		Environment: new(event_kit_api.Environment{Name: "prod"}),
		Principal:   event_kit_api.BatchPrincipal{PrincipalType: "batch", Username: "scheduler"},
	})
	assert.Equal(t, map[string]string{
		"steadybit.environment.name":   "prod",
		"steadybit.principal.type":     "batch",
		"steadybit.principal.username": "scheduler",
	}, props)
}

func Test_addProperties_IgnoreNil(t *testing.T) {
	props := make(map[string]string)
	addExperimentExecutionProperties(props, nil)
	addStepExecutionProperties(props, nil)
	addTargetExecutionProperties(props, nil)
	assert.Empty(t, props)
}

func Test_onExperimentStarted(t *testing.T) {
	eventTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err := onExperimentStarted(&event_kit_api.EventRequestBody{
		Environment: new(event_kit_api.Environment{Name: "prod"}),
		EventTime:   eventTime,
		ExperimentExecution: new(event_kit_api.ExperimentExecution{
			ExecutionId:   7,
			ExperimentKey: "EXP-1",
			Name:          "My Experiment",
			State:         event_kit_api.ExperimentExecutionStateRunning,
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, &types.EventIngest{
		EventType: "CUSTOM_INFO",
		Title:     "Steadybit experiment 'EXP-1 / 7' started",
		Properties: map[string]string{
			"steadybit.environment.name": "prod",
			"steadybit.experiment.key":   "EXP-1",
			"steadybit.experiment.name":  "My Experiment",
			"steadybit.execution.id":     "7",
			"steadybit.execution.state":  "running",
		},
		StartTime: new(eventTime.UnixMilli()),
		EndTime:   new(eventTime.UnixMilli()),
	}, got)
}

func Test_onExperimentCompleted_RemovesStepExecutions(t *testing.T) {
	endedTime := time.Date(2021, 1, 1, 0, 5, 0, 0, time.UTC)
	ownStep := event_kit_api.ExperimentStepExecution{Id: uuid.New(), ExecutionId: 7}
	otherStep := event_kit_api.ExperimentStepExecution{Id: uuid.New(), ExecutionId: 8}
	storeStepExecution(t, ownStep)
	storeStepExecution(t, otherStep)

	got, err := onExperimentCompleted(&event_kit_api.EventRequestBody{
		Environment: new(event_kit_api.Environment{Name: "prod"}),
		ExperimentExecution: new(event_kit_api.ExperimentExecution{
			ExecutionId:   7,
			ExperimentKey: "EXP-1",
			Name:          "My Experiment",
			State:         event_kit_api.ExperimentExecutionStateCompleted,
			EndedTime:     &endedTime,
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, "Steadybit experiment 'EXP-1 / 7' ended", got.Title)
	assert.Equal(t, "completed", got.Properties["steadybit.execution.state"])
	assert.Equal(t, new(endedTime.UnixMilli()), got.StartTime)
	assert.Equal(t, new(endedTime.UnixMilli()), got.EndTime)

	_, ownStillPresent := stepExecutions.Load(ownStep.Id)
	_, otherStillPresent := stepExecutions.Load(otherStep.Id)
	assert.False(t, ownStillPresent, "step execution of completed experiment should be removed")
	assert.True(t, otherStillPresent, "step execution of other experiment should be kept")
}

func Test_onExperimentStepStarted_MissingStepExecution(t *testing.T) {
	got, err := onExperimentStepStarted(&event_kit_api.EventRequestBody{})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func Test_onExperimentTargetCompleted(t *testing.T) {
	useEntityCache(t, map[string]string{"type(HOST),entityName.equals(host-1)": "HOST-1"})
	startedTime := time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)
	endedTime := time.Date(2021, 1, 1, 0, 3, 0, 0, time.UTC)
	step := event_kit_api.ExperimentStepExecution{
		Id:          uuid.New(),
		ExecutionId: 7,
		Type:        event_kit_api.Action,
		ActionId:    new("com.steadybit.extension_host.stress-cpu"),
		ActionName:  new("Stress CPU"),
		ActionKind:  new(event_kit_api.Attack),
	}
	storeStepExecution(t, step)

	got, err := onExperimentTargetCompleted(&event_kit_api.EventRequestBody{
		Environment: new(event_kit_api.Environment{Name: "prod"}),
		ExperimentStepTargetExecution: new(event_kit_api.ExperimentStepTargetExecution{
			ExecutionId:      7,
			ExperimentKey:    "EXP-1",
			StepExecutionId:  step.Id,
			State:            "completed",
			TargetType:       "com.steadybit.extension_host.host",
			TargetName:       "host-1",
			TargetAttributes: map[string][]string{"host.hostname": {"host-1"}},
			StartedTime:      &startedTime,
			EndedTime:        &endedTime,
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, &types.EventIngest{
		EventType: "CUSTOM_INFO",
		Title:     "Steadybit experiment 'EXP-1 / 7' - Attack 'Stress CPU' ended - Target 'host-1'",
		Properties: map[string]string{
			"steadybit.environment.name":       "prod",
			"steadybit.experiment.key":         "EXP-1",
			"steadybit.execution.id":           "7",
			"steadybit.execution.target.state": "completed",
			"steadybit.step.action.id":         "com.steadybit.extension_host.stress-cpu",
			"steadybit.step.action.name":       "Stress CPU",
		},
		EntitySelector: new("type(HOST),entityName.equals(host-1)"),
		StartTime:      new(endedTime.UnixMilli()),
		EndTime:        new(endedTime.UnixMilli()),
	}, got)
}

func Test_onExperimentTargetEvents_WithoutEvent(t *testing.T) {
	nonAttackStep := event_kit_api.ExperimentStepExecution{
		Id:         uuid.New(),
		Type:       event_kit_api.Action,
		ActionId:   new("com.steadybit.extension_http.check"),
		ActionKind: new(event_kit_api.Check),
	}
	storeStepExecution(t, nonAttackStep)

	tests := []struct {
		name    string
		event   event_kit_api.EventRequestBody
		wantErr bool
	}{
		{
			name:    "missing target execution",
			event:   event_kit_api.EventRequestBody{},
			wantErr: true,
		},
		{
			name: "unknown step execution",
			event: event_kit_api.EventRequestBody{
				ExperimentStepTargetExecution: new(event_kit_api.ExperimentStepTargetExecution{StepExecutionId: uuid.New()}),
			},
		},
		{
			name: "step is not an attack",
			event: event_kit_api.EventRequestBody{
				ExperimentStepTargetExecution: new(event_kit_api.ExperimentStepTargetExecution{StepExecutionId: nonAttackStep.Id}),
			},
		},
	}

	handlers := map[string]eventHandler{
		"onExperimentTargetStarted":   onExperimentTargetStarted,
		"onExperimentTargetCompleted": onExperimentTargetCompleted,
	}
	for handlerName, handler := range handlers {
		for _, tt := range tests {
			t.Run(handlerName+"/"+tt.name, func(t *testing.T) {
				got, err := handler(&tt.event)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				assert.Nil(t, got)
			})
		}
	}
}

func Test_handle(t *testing.T) {
	t.Run("valid body", func(t *testing.T) {
		called := make(chan *event_kit_api.EventRequestBody, 1)
		h := handle(func(event *event_kit_api.EventRequestBody) (*types.EventIngest, error) {
			called <- event
			return nil, nil
		})

		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodPost, "/events/experiment-started", nil), []byte(`{"eventName":"experiment.started"}`))

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `"{}"`, w.Body.String())
		select {
		case event := <-called:
			assert.Equal(t, "experiment.started", event.EventName)
		case <-time.After(5 * time.Second):
			t.Fatal("handler was not called")
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		h := handle(func(event *event_kit_api.EventRequestBody) (*types.EventIngest, error) {
			t.Error("handler must not be called for invalid body")
			return nil, nil
		})

		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodPost, "/events/experiment-started", nil), []byte(`not json`))

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to decode event request body")
	})
}

type mockPostEventApi struct {
	result   *types.EventIngestResults
	response *http.Response
	err      error
	calls    int
}

func (m *mockPostEventApi) PostEvent(_ context.Context, _ types.EventIngest) (*types.EventIngestResults, *http.Response, error) {
	m.calls++
	return m.result, m.response, m.err
}

func (m *mockPostEventApi) GetEntities(_ context.Context, _ string) (*types.EntitiesList, *http.Response, error) {
	return nil, nil, errors.New("not implemented")
}

func Test_sendDynatraceEvent(t *testing.T) {
	tests := []struct {
		name string
		api  *mockPostEventApi
	}{
		{name: "error", api: &mockPostEventApi{err: errors.New("boom")}},
		{name: "unexpected status code", api: &mockPostEventApi{response: &http.Response{StatusCode: 400}}},
		{name: "event created", api: &mockPostEventApi{response: &http.Response{StatusCode: 201}, result: &types.EventIngestResults{ReportCount: 1}}},
		{name: "no event created", api: &mockPostEventApi{response: &http.Response{StatusCode: 201}, result: &types.EventIngestResults{ReportCount: 0}}},
		{name: "no result", api: &mockPostEventApi{response: &http.Response{StatusCode: 201}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				sendDynatraceEvent(tt.api, &types.EventIngest{Title: "test"})
			})
			assert.Equal(t, 1, tt.api.calls)
		})
	}
}

func Test_RegisterEventListenerHandlers_EntityLoader(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("entitySelector") {
		case "single":
			_, _ = w.Write([]byte(`{"entities":[{"entityId":"HOST-1"}]}`))
		case "multiple":
			_, _ = w.Write([]byte(`{"entities":[{"entityId":"HOST-1"},{"entityId":"HOST-2"}]}`))
		case "none":
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case "invalid":
			_, _ = w.Write([]byte(`not json`))
		default:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	previousConfig := config.Config
	previousCache := entityCache
	config.Config = config.Specification{ApiBaseUrl: srv.URL, ApiToken: "token"}
	t.Cleanup(func() {
		entityCache.Stop()
		config.Config = previousConfig
		entityCache = previousCache
	})

	RegisterEventListenerHandlers()

	assert.Equal(t, "HOST-1", entityCache.Get("single").Value())
	assert.Equal(t, "", entityCache.Get("multiple").Value())
	assert.Equal(t, "", entityCache.Get("none").Value())
	assert.Equal(t, "", entityCache.Get("invalid").Value())
	assert.Equal(t, "", entityCache.Get("forbidden").Value())

	// results, including empty ones, are cached
	requestsBefore := requests.Load()
	entityCache.Get("single")
	entityCache.Get("none")
	assert.Equal(t, requestsBefore, requests.Load())
}
