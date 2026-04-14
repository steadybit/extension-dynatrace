// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extproblems

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-dynatrace/config"
	"github.com/steadybit/extension-dynatrace/types"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type ProblemCheckAction struct{}

// Make sure action implements all required interfaces
var (
	_ action_kit_sdk.Action[ProblemCheckState]           = (*ProblemCheckAction)(nil)
	_ action_kit_sdk.ActionWithStatus[ProblemCheckState] = (*ProblemCheckAction)(nil)
)

type ProblemCheckState struct {
	Start                 time.Time
	End                   time.Time
	EntitySelector        *string
	Condition             string
	ConditionCheckMode    string
	ConditionCheckSuccess bool
}

func NewProblemCheckAction() action_kit_sdk.Action[ProblemCheckState] {
	return &ProblemCheckAction{}
}

func (m *ProblemCheckAction) NewEmptyState() ProblemCheckState {
	return ProblemCheckState{}
}

func (m *ProblemCheckAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          ProblemCheckActionId,
		Label:       "Problem Check",
		Description: "Checks for the existence of open problems in Dynatrace.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(problemCheckActionIcon),
		Technology:  new("Dynatrace"),

		Kind:        action_kit_api.Check,
		TimeControl: action_kit_api.TimeControlInternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new(""),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("30s"),
				Order:        new(1),
				Required:     new(true),
			},
			{
				Name:        "entitySelector",
				Label:       "Entity Selector",
				Description: new("Filter Problems by an Dynatrace entity selector. If empty, all problems are considered."),
				Type:        action_kit_api.ActionParameterTypeString,
				Order:       new(2),
				Required:    new(false),
			},
			{
				Name:        "condition",
				Label:       "Condition",
				Description: new(""),
				Type:        action_kit_api.ActionParameterTypeString,
				Options: new([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "No check, only show problems",
						Value: conditionShowOnly,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "No problem expected",
						Value: conditionNoProblems,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "At least one problem expected",
						Value: conditionAtLeastOneProblem,
					},
				}),
				DefaultValue: new(conditionShowOnly),
				Order:        new(3),
				Required:     new(true),
			},
			{
				Name:         "conditionCheckMode",
				Label:        "Condition Check Mode",
				Description:  new("Should the step succeed if the condition is met at least once or all the time?"),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: new(conditionCheckModeAllTheTime),
				Options: new([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "All the time",
						Value: conditionCheckModeAllTheTime,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "At least once",
						Value: conditionCheckModeAtLeastOnce,
					},
				}),
				Required: new(true),
				Order:    new(4),
			},
		},
		Widgets: new([]action_kit_api.Widget{
			action_kit_api.StateOverTimeWidget{
				Type:  action_kit_api.ComSteadybitWidgetStateOverTime,
				Title: "Dynatrace Problems",
				Identity: action_kit_api.StateOverTimeWidgetIdentityConfig{
					From: "dynatrace.problem.id",
				},
				Label: action_kit_api.StateOverTimeWidgetLabelConfig{
					From: "dynatrace.problem.title",
				},
				State: action_kit_api.StateOverTimeWidgetStateConfig{
					From: "state",
				},
				Tooltip: action_kit_api.StateOverTimeWidgetTooltipConfig{
					From: "tooltip",
				},
				Url: new(action_kit_api.StateOverTimeWidgetUrlConfig{
					From: new("url"),
				}),
				Value: new(action_kit_api.StateOverTimeWidgetValueConfig{
					Hide: new(true),
				}),
			},
		}),
		Prepare: action_kit_api.MutatingEndpointReference{},
		Start:   action_kit_api.MutatingEndpointReference{},
		Status: new(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: new("5s"),
		}),
	}
}

func (m *ProblemCheckAction) Prepare(_ context.Context, state *ProblemCheckState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	duration := request.Config["duration"].(float64)
	state.Start = time.Now()
	state.End = time.Now().Add(time.Millisecond * time.Duration(duration))

	if extutil.ToString(request.Config["entitySelector"]) != "" {
		state.EntitySelector = new(extutil.ToString(request.Config["entitySelector"]))
	}

	if request.Config["condition"] != nil {
		state.Condition = fmt.Sprintf("%v", request.Config["condition"])
	}

	if request.Config["conditionCheckMode"] != nil {
		state.ConditionCheckMode = fmt.Sprintf("%v", request.Config["conditionCheckMode"])
	}

	return nil, nil
}

func (m *ProblemCheckAction) Start(ctx context.Context, state *ProblemCheckState) (*action_kit_api.StartResult, error) {
	statusResult, err := ProblemCheckStatus(ctx, state, &config.Config)
	if statusResult == nil {
		return nil, err
	}
	startResult := action_kit_api.StartResult{
		Artifacts: statusResult.Artifacts,
		Error:     statusResult.Error,
		Messages:  statusResult.Messages,
		Metrics:   statusResult.Metrics,
	}
	return &startResult, err
}

func (m *ProblemCheckAction) Status(ctx context.Context, state *ProblemCheckState) (*action_kit_api.StatusResult, error) {
	return ProblemCheckStatus(ctx, state, &config.Config)
}

type ProblemsApi interface {
	GetProblems(ctx context.Context, from time.Time, entitySelector *string) ([]types.Problem, *http.Response, error)
}

func ProblemCheckStatus(ctx context.Context, state *ProblemCheckState, api ProblemsApi) (*action_kit_api.StatusResult, error) {
	now := time.Now()
	problems, _, err := api.GetProblems(ctx, state.Start, state.EntitySelector)
	if err != nil {
		return nil, extension_kit.ToError("Failed to get problems from Dynatrace.", err)
	}

	completed := now.After(state.End)
	var checkError *action_kit_api.ActionKitError
	if state.ConditionCheckMode == conditionCheckModeAllTheTime {
		if state.Condition == conditionNoProblems && len(problems) > 0 {
			checkError = new(action_kit_api.ActionKitError{
				Title:  fmt.Sprintf("No problem expected, but %d problems found.", len(problems)),
				Status: extutil.Ptr(action_kit_api.Failed),
			})
		}
		if state.Condition == conditionAtLeastOneProblem && len(problems) == 0 {
			checkError = new(action_kit_api.ActionKitError{
				Title:  "At least one problem expected, but no problems found.",
				Status: extutil.Ptr(action_kit_api.Failed),
			})
		}

	} else if state.ConditionCheckMode == conditionCheckModeAtLeastOnce {
		if state.Condition == conditionNoProblems && len(problems) == 0 {
			state.ConditionCheckSuccess = true
		}
		if state.Condition == conditionAtLeastOneProblem && len(problems) > 0 {
			state.ConditionCheckSuccess = true
		}
		if completed && !state.ConditionCheckSuccess {
			if state.Condition == conditionNoProblems {
				checkError = new(action_kit_api.ActionKitError{
					Title:  "No problem expected, but problems found.",
					Status: extutil.Ptr(action_kit_api.Failed),
				})
			} else if state.Condition == conditionAtLeastOneProblem {
				checkError = new(action_kit_api.ActionKitError{
					Title:  "At least one problem expected, but no problems found.",
					Status: extutil.Ptr(action_kit_api.Failed),
				})
			}
		}
	}

	var metrics []action_kit_api.Metric
	for _, problem := range problems {
		metrics = append(metrics, toMetric(problem, now))
	}

	return &action_kit_api.StatusResult{
		Completed: completed,
		Error:     checkError,
		Metrics:   new(metrics),
	}, nil
}

func toMetric(problem types.Problem, now time.Time) action_kit_api.Metric {
	var tooltip strings.Builder
	tooltip.WriteString(problem.DisplayId)
	for _, entity := range problem.AffectedEntities {
		tooltip.WriteString(fmt.Sprintf("\n- %s", entity.Name))
	}

	return action_kit_api.Metric{
		Name: new("dynatrace_problems"),
		Metric: map[string]string{
			"dynatrace.problem.id":    problem.ProblemId,
			"dynatrace.problem.title": problem.Title,
			"state":                   "danger",
			"tooltip":                 tooltip.String(),
			"url":                     fmt.Sprintf("%s%s;pid=%s", config.Config.UiBaseUrl, config.Config.UiProblemsPath, problem.ProblemId),
		},
		Timestamp: now,
		Value:     0,
	}
}
