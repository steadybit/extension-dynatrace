// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extproblems

import (
	"context"
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-dynatrace/config"
	"github.com/steadybit/extension-dynatrace/types"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"time"
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
		Icon:        extutil.Ptr(problemCheckActionIcon),
		Category:    extutil.Ptr("monitoring"),
		Kind:        action_kit_api.Check,
		TimeControl: action_kit_api.TimeControlInternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr(""),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("30s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
			{
				Name:        "entitySelector",
				Label:       "Entity Selector",
				Description: extutil.Ptr("Filter Problems by an Dynatrace entity selector. If empty, all problems are considered."),
				Type:        action_kit_api.String,
				Order:       extutil.Ptr(2),
				Required:    extutil.Ptr(false),
			},
			{
				Name:        "condition",
				Label:       "Condition",
				Description: extutil.Ptr(""),
				Type:        action_kit_api.String,
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
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
				DefaultValue: extutil.Ptr(conditionShowOnly),
				Order:        extutil.Ptr(3),
				Required:     extutil.Ptr(true),
			},
			{
				Name:         "conditionCheckMode",
				Label:        "Condition Check Mode",
				Description:  extutil.Ptr("Should the step succeed if the condition is met at least once or all the time?"),
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr(conditionCheckModeAllTheTime),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "All the time",
						Value: conditionCheckModeAllTheTime,
					},
					action_kit_api.ExplicitParameterOption{
						Label: "At least once",
						Value: conditionCheckModeAtLeastOnce,
					},
				}),
				Required: extutil.Ptr(true),
				Order:    extutil.Ptr(4),
			},
		},
		Widgets: extutil.Ptr([]action_kit_api.Widget{
			action_kit_api.StateOverTimeWidget{
				Type:  action_kit_api.ComSteadybitWidgetStateOverTime,
				Title: "Dynatrace Problems",
				Identity: action_kit_api.StateOverTimeWidgetIdentityConfig{
					From: "dynatrace.problem.id",
				},
				Label: action_kit_api.StateOverTimeWidgetLabelConfig{
					From: "dynatrace.problem.displayId",
				},
				State: action_kit_api.StateOverTimeWidgetStateConfig{
					From: "state",
				},
				Tooltip: action_kit_api.StateOverTimeWidgetTooltipConfig{
					From: "tooltip",
				},
				Url: extutil.Ptr(action_kit_api.StateOverTimeWidgetUrlConfig{
					From: extutil.Ptr("url"),
				}),
				Value: extutil.Ptr(action_kit_api.StateOverTimeWidgetValueConfig{
					Hide: extutil.Ptr(true),
				}),
			},
		}),
		Prepare: action_kit_api.MutatingEndpointReference{},
		Start:   action_kit_api.MutatingEndpointReference{},
		Status: extutil.Ptr(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("5s"),
		}),
	}
}

func (m *ProblemCheckAction) Prepare(_ context.Context, state *ProblemCheckState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	duration := request.Config["duration"].(float64)
	state.Start = time.Now()
	state.End = time.Now().Add(time.Millisecond * time.Duration(duration))

	if request.Config["entitySelector"] != nil {
		state.EntitySelector = extutil.Ptr(fmt.Sprintf("%v", request.Config["entitySelector"]))
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
	problems, resp, err := api.GetProblems(ctx, state.Start, state.EntitySelector)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to get problems from Dynatrace. Full response: %v", resp), err)
	}

	completed := now.After(state.End)
	var checkError *action_kit_api.ActionKitError
	if state.ConditionCheckMode == conditionCheckModeAllTheTime {
		if state.Condition == conditionNoProblems && len(problems) > 0 {
			checkError = extutil.Ptr(action_kit_api.ActionKitError{
				Title:  fmt.Sprintf("No problem expected, but %d problems found.", len(problems)),
				Status: extutil.Ptr(action_kit_api.Failed),
			})
		}
		if state.Condition == conditionAtLeastOneProblem && len(problems) == 0 {
			checkError = extutil.Ptr(action_kit_api.ActionKitError{
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
				checkError = extutil.Ptr(action_kit_api.ActionKitError{
					Title:  "No problem expected, but problems found.",
					Status: extutil.Ptr(action_kit_api.Failed),
				})
			} else if state.Condition == conditionAtLeastOneProblem {
				checkError = extutil.Ptr(action_kit_api.ActionKitError{
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
		Metrics:   extutil.Ptr(metrics),
	}, nil
}

func toMetric(problem types.Problem, now time.Time) action_kit_api.Metric {
	tooltip := problem.Title
	for _, entity := range problem.AffectedEntities {
		tooltip += fmt.Sprintf("\n- %s", entity.Name)
	}

	return action_kit_api.Metric{
		Name: extutil.Ptr("dynatrace_problems"),
		Metric: map[string]string{
			"dynatrace.problem.id":        problem.ProblemId,
			"dynatrace.problem.displayId": problem.DisplayId,
			"state":                       "danger",
			"tooltip":                     tooltip,
			"url":                         fmt.Sprintf("%s/ui/apps/dynatrace.classic.problems/#problems/problemdetails;pid=%s", config.Config.ApiBaseUrl[:len(config.Config.ApiBaseUrl)-len("/api")], problem.ProblemId),
		},
		Timestamp: now,
		Value:     0,
	}
}
