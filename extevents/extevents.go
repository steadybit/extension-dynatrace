// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extevents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/event-kit/go/event_kit_api"
	"github.com/steadybit/extension-dynatrace/config"
	"github.com/steadybit/extension-dynatrace/types"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"sync"
)

func RegisterEventListenerHandlers() {
	exthttp.RegisterHttpHandler("/events/experiment-started", handle(onExperimentStarted))
	exthttp.RegisterHttpHandler("/events/experiment-completed", handle(onExperimentCompleted))
	exthttp.RegisterHttpHandler("/events/experiment-step-started", handle(onExperimentStepStarted))
	exthttp.RegisterHttpHandler("/events/experiment-step-completed", handle(onExperimentStepCompleted))
	exthttp.RegisterHttpHandler("/events/experiment-target-started", handle(onExperimentTargetStarted))
	exthttp.RegisterHttpHandler("/events/experiment-target-completed", handle(onExperimentTargetCompleted))
}

type PostEventApi interface {
	PostEvent(ctx context.Context, event types.EventIngest) (*types.EventIngestResults, *http.Response, error)
}

var (
	stepExecutions = sync.Map{}
)

type eventHandler func(event *event_kit_api.EventRequestBody) (*types.EventIngest, error)

func handle(handler eventHandler) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {

		event, err := parseBodyToEventRequestBody(body)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to decode event request body", err))
			return
		}

		if request, err := handler(&event); err == nil {
			if request != nil {
				sendDynatraceEvent(r.Context(), &config.Config, request)
			}
		} else {
			exthttp.WriteError(w, extension_kit.ToError(err.Error(), err))
			return
		}

		exthttp.WriteBody(w, "{}")
	}
}

func onExperimentStarted(event *event_kit_api.EventRequestBody) (*types.EventIngest, error) {
	props := make(map[string]string)
	addBaseProperties(props, event)
	addExperimentExecutionProperties(props, event.ExperimentExecution)
	return &types.EventIngest{
		EventType:  "CUSTOM_INFO",
		Title:      fmt.Sprintf("Steadybit experiment '%s / %g' started", event.ExperimentExecution.ExperimentKey, event.ExperimentExecution.ExecutionId),
		Properties: props,
		StartTime:  extutil.Ptr(event.ExperimentExecution.StartedTime.Unix()),
		EndTime:    extutil.Ptr(event.ExperimentExecution.StartedTime.Unix()),
	}, nil
}

func onExperimentCompleted(event *event_kit_api.EventRequestBody) (*types.EventIngest, error) {
	props := make(map[string]string)
	addBaseProperties(props, event)
	addExperimentExecutionProperties(props, event.ExperimentExecution)
	return &types.EventIngest{
		EventType:  "CUSTOM_INFO",
		Title:      fmt.Sprintf("Steadybit experiment '%s / %g' ended", event.ExperimentExecution.ExperimentKey, event.ExperimentExecution.ExecutionId),
		Properties: props,
		StartTime:  extutil.Ptr(event.ExperimentExecution.EndedTime.Unix()),
		EndTime:    extutil.Ptr(event.ExperimentExecution.EndedTime.Unix()),
	}, nil
}

func onExperimentStepStarted(event *event_kit_api.EventRequestBody) (*types.EventIngest, error) {
	if event.ExperimentStepExecution == nil {
		return nil, errors.New("missing ExperimentStepExecution in event")
	}
	if event.ExperimentStepExecution.ActionKind != nil && *event.ExperimentStepExecution.ActionKind == event_kit_api.Attack {
		stepExecutions.Store(event.ExperimentStepExecution.Id, *event.ExperimentStepExecution)
	}
	return nil, nil
}

func onExperimentStepCompleted(event *event_kit_api.EventRequestBody) (*types.EventIngest, error) {
	if event.ExperimentStepExecution == nil {
		return nil, errors.New("missing ExperimentStepExecution in event")
	}
	stepExecutions.Delete(event.ExperimentStepExecution.Id)
	return nil, nil
}

func onExperimentTargetStarted(event *event_kit_api.EventRequestBody) (*types.EventIngest, error) {
	if event.ExperimentStepTargetExecution == nil {
		return nil, errors.New("missing ExperimentStepTargetExecution in event")
	}

	var v, ok = stepExecutions.Load(event.ExperimentStepTargetExecution.StepExecutionId)
	if !ok {
		log.Warn().Msgf("Could not find step infos for step execution id %s", event.ExperimentStepTargetExecution.StepExecutionId)
		return nil, nil
	}
	stepExecution := v.(event_kit_api.ExperimentStepExecution)

	props := make(map[string]string)
	addBaseProperties(props, event)
	addStepExecutionProperties(props, &stepExecution)
	addTargetExecutionProperties(props, event.ExperimentStepTargetExecution)

	return &types.EventIngest{
		EventType: "CUSTOM_INFO",
		Title: fmt.Sprintf("Steadybit experiment '%s / %g' - Attack '%s' started - Target '%s'",
			event.ExperimentStepTargetExecution.ExperimentKey,
			event.ExperimentStepTargetExecution.ExecutionId,
			getActionName(stepExecution),
			getTargetName(*event.ExperimentStepTargetExecution)),
		Properties:     props,
		EntitySelector: getEntitySelector(*event.ExperimentStepTargetExecution),
		StartTime:      extutil.Ptr(event.ExperimentStepTargetExecution.StartedTime.Unix()),
		EndTime:        extutil.Ptr(event.ExperimentStepTargetExecution.StartedTime.Unix()),
	}, nil
}

func onExperimentTargetCompleted(event *event_kit_api.EventRequestBody) (*types.EventIngest, error) {
	if event.ExperimentStepTargetExecution == nil {
		return nil, errors.New("missing ExperimentStepTargetExecution in event")
	}

	var v, ok = stepExecutions.Load(event.ExperimentStepTargetExecution.StepExecutionId)
	if !ok {
		log.Warn().Msgf("Could not find step infos for step execution id %s", event.ExperimentStepTargetExecution.StepExecutionId)
		return nil, nil
	}
	stepExecution := v.(event_kit_api.ExperimentStepExecution)

	props := make(map[string]string)
	addBaseProperties(props, event)
	addStepExecutionProperties(props, &stepExecution)
	addTargetExecutionProperties(props, event.ExperimentStepTargetExecution)

	return &types.EventIngest{
		EventType: "CUSTOM_INFO",
		Title: fmt.Sprintf("Steadybit experiment '%s / %g' - Attack '%s' ended - Target '%s'",
			event.ExperimentStepTargetExecution.ExperimentKey,
			event.ExperimentStepTargetExecution.ExecutionId,
			getActionName(stepExecution),
			getTargetName(*event.ExperimentStepTargetExecution)),
		Properties:     props,
		EntitySelector: getEntitySelector(*event.ExperimentStepTargetExecution),
		StartTime:      extutil.Ptr(event.ExperimentStepTargetExecution.EndedTime.Unix()),
		EndTime:        extutil.Ptr(event.ExperimentStepTargetExecution.EndedTime.Unix()),
	}, nil
}

func getActionName(target event_kit_api.ExperimentStepExecution) string {
	actionName := *target.ActionId
	if target.ActionName != nil {
		actionName = *target.ActionName
	}
	if target.CustomLabel != nil {
		actionName = *target.CustomLabel
	}
	return actionName
}

func getTargetName(target event_kit_api.ExperimentStepTargetExecution) string {
	if values, ok := target.TargetAttributes["steadybit.label"]; ok {
		return values[0]
	}
	return target.TargetName
}

func getEntitySelector(target event_kit_api.ExperimentStepTargetExecution) *string {
	if target.TargetType == "com.steadybit.extension_kubernetes.kubernetes-cluster" && hasSingleAttribute(target, "k8s.cluster-name") {
		return extutil.Ptr(fmt.Sprintf("type(\"KUBERNETES_CLUSTER\"),entityName.equals(\"%s\")", target.TargetAttributes["k8s.cluster-name"][0]))
	} else if target.TargetType == "com.steadybit.extension_kubernetes.kubernetes-deployment" && hasSingleAttribute(target, "k8s.deployment") {
		return extutil.Ptr(fmt.Sprintf("type(\"CLOUD_APPLICATION\"),entityName.equals(\"%s\")", target.TargetAttributes["k8s.deployment"][0]))
	} else if target.TargetType == "com.steadybit.extension_kubernetes.kubernetes-statefulset" && hasSingleAttribute(target, "k8s.statefulset") {
		return extutil.Ptr(fmt.Sprintf("type(\"CLOUD_APPLICATION\"),entityName.equals(\"%s\")", target.TargetAttributes["k8s.statefulset"][0]))
	} else if target.TargetType == "com.steadybit.extension_kubernetes.kubernetes-daemonset" && hasSingleAttribute(target, "k8s.daemonset") {
		return extutil.Ptr(fmt.Sprintf("type(\"CLOUD_APPLICATION\"),entityName.equals(\"%s\")", target.TargetAttributes["k8s.daemonset"][0]))
	} else if target.TargetType == "com.steadybit.extension_kubernetes.kubernetes-node" && hasSingleAttribute(target, "k8s.node.name") {
		return extutil.Ptr(fmt.Sprintf("type(\"KUBERNETES_NODE\"),entityName.equals(\"%s\")", target.TargetAttributes["k8s.node.name"][0]))
	} else if target.TargetType == "com.steadybit.extension_kubernetes.kubernetes-pod" && hasSingleAttribute(target, "k8s.pod.name") {
		return extutil.Ptr(fmt.Sprintf("type(\"CLOUD_APPLICATION_INSTANCE\"),entityName.equals(\"%s\")", target.TargetAttributes["k8s.pod.name"][0]))
	} else if target.TargetType == "com.steadybit.extension_jvm.application" && hasSingleAttribute(target, "k8s.pod.name") {
		return extutil.Ptr(fmt.Sprintf("type(\"CLOUD_APPLICATION_INSTANCE\"),entityName.equals(\"%s\")", target.TargetAttributes["k8s.pod.name"][0]))
	} else if target.TargetType == "com.steadybit.extension_container.container" && hasSingleAttribute(target, "k8s.container.name") && hasSingleAttribute(target, "k8s.pod.name") {
		return extutil.Ptr(fmt.Sprintf("type(\"CONTAINER_GROUP_INSTANCE\"),entityName.equals(\"%s %s\")", target.TargetAttributes["k8s.pod.name"][0], target.TargetAttributes["k8s.container.name"][0]))
	} else if target.TargetType == "com.steadybit.extension_host.host" && hasSingleAttribute(target, "host.hostname") {
		if hasSingleAttribute(target, "k8s.cluster-name") {
			return extutil.Ptr(fmt.Sprintf("type(\"KUBERNETES_NODE\"),entityName.equals(\"%s\")", target.TargetAttributes["host.hostname"][0]))
		} else if hasSingleAttribute(target, "host.hostname") {
			return extutil.Ptr(fmt.Sprintf("type(\"HOST\"),entityName.equals(\"%s\")", target.TargetAttributes["host.hostname"][0]))
		}
	}
	return nil
}

func hasSingleAttribute(target event_kit_api.ExperimentStepTargetExecution, attribute string) bool {
	if values, ok := target.TargetAttributes[attribute]; ok {
		return len(values) == 1
	}
	return false
}

func addBaseProperties(props map[string]string, event *event_kit_api.EventRequestBody) {
	props["steadybit.environment.name"] = event.Environment.Name
	if event.Team != nil {
		props["steadybit.team.name"] = event.Team.Name
		props["steadybit.team.key"] = event.Team.Key
	}
	userPrincipal, isUserPrincipal := event.Principal.(event_kit_api.UserPrincipal)
	if isUserPrincipal {
		props["steadybit.principal.type"] = userPrincipal.PrincipalType
		props["steadybit.principal.username"] = userPrincipal.Username
		props["steadybit.principal.name"] = userPrincipal.Name
	}
	accessTokenPrincipal, isAccessTokenPrincipal := event.Principal.(event_kit_api.AccessTokenPrincipal)
	if isAccessTokenPrincipal {
		props["steadybit.principal.type"] = accessTokenPrincipal.PrincipalType
		props["steadybit.principal.name"] = accessTokenPrincipal.Name
	}
	batchPrincipal, isBatchPrincipal := event.Principal.(event_kit_api.BatchPrincipal)
	if isBatchPrincipal {
		props["steadybit.principal.type"] = batchPrincipal.PrincipalType
		props["steadybit.principal.username"] = batchPrincipal.Username
	}
}

func addExperimentExecutionProperties(props map[string]string, experimentExecution *event_kit_api.ExperimentExecution) {
	if experimentExecution == nil {
		return
	}
	props["steadybit.experiment.key"] = experimentExecution.ExperimentKey
	props["steadybit.experiment.name"] = experimentExecution.Name
	props["steadybit.execution.id"] = fmt.Sprintf("%g", experimentExecution.ExecutionId)
	props["steadybit.execution.state"] = string(experimentExecution.State)
	if len(experimentExecution.Hypothesis) > 0 {
		props["steadybit.experiment.hypothesis"] = experimentExecution.Hypothesis
	}
}

func addStepExecutionProperties(props map[string]string, stepExecution *event_kit_api.ExperimentStepExecution) {
	if stepExecution == nil {
		return
	}
	if stepExecution.Type == event_kit_api.Action {
		props["steadybit.step.action.id"] = *stepExecution.ActionId
	}
	if stepExecution.ActionName != nil {
		props["steadybit.step.action.name"] = *stepExecution.ActionName
	}
	if stepExecution.CustomLabel != nil {
		props["steadybit.step.action.custom_label"] = *stepExecution.CustomLabel
	}
}

func addTargetExecutionProperties(props map[string]string, targetExecution *event_kit_api.ExperimentStepTargetExecution) {
	if targetExecution == nil {
		return
	}
	props["steadybit.experiment.key"] = targetExecution.ExperimentKey
	props["steadybit.execution.id"] = fmt.Sprintf("%g", targetExecution.ExecutionId)
	props["steadybit.execution.target.state"] = string(targetExecution.State)

	addIfPresent(props, *targetExecution, "k8s.cluster-name", "dt.entity.kubernetes_cluster")
	addIfPresent(props, *targetExecution, "k8s.namespace", "dt.entity.cloud_application_namespace")
	addIfPresent(props, *targetExecution, "k8s.deployment", "dt.entity.cloud_application")
	addIfPresent(props, *targetExecution, "k8s.pod.name", "dt.entity.cloud_application_instance")
	if _, ok := targetExecution.TargetAttributes["k8s.cluster-name"]; ok {
		addIfPresent(props, *targetExecution, "container.host", "dt.entity.kubernetes_node")
		addIfPresent(props, *targetExecution, "host.hostname", "dt.entity.kubernetes_node")
		addIfPresent(props, *targetExecution, "application.hostname", "dt.entity.kubernetes_node")
		addIfPresent(props, *targetExecution, "k8s.node.name", "dt.entity.kubernetes_node")
	}
}

func addIfPresent(props map[string]string, target event_kit_api.ExperimentStepTargetExecution, steadybitAttribute string, dynatraceProperty string) {
	if values, ok := target.TargetAttributes[steadybitAttribute]; ok {
		//We don't want to add one-to-many attributes to dynatrace. For example when attacking a host, we don't want to add all namespaces or pods which are running on that host.
		if (len(values)) == 1 {
			props[dynatraceProperty] = values[0]
		}
	}
}

func parseBodyToEventRequestBody(body []byte) (event_kit_api.EventRequestBody, error) {
	var event event_kit_api.EventRequestBody
	err := json.Unmarshal(body, &event)
	return event, err
}

func sendDynatraceEvent(ctx context.Context, api PostEventApi, event *types.EventIngest) {
	_, r, err := api.PostEvent(ctx, *event)

	if err != nil {
		log.Err(err).Msgf("Failed to send Dynatrace event. Full response %v", r)
	} else if r.StatusCode != 201 {
		log.Error().Msgf("Dynatrace API responded with unexpected status code %d while sending Event. Full response: %v",
			r.StatusCode, r)
	}
}
