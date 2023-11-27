// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extmaintenance

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

type CreateMaintenanceWindowAction struct{}

// Make sure action implements all required interfaces
var (
	_ action_kit_sdk.Action[CreateMaintenanceWindowState]         = (*CreateMaintenanceWindowAction)(nil)
	_ action_kit_sdk.ActionWithStop[CreateMaintenanceWindowState] = (*CreateMaintenanceWindowAction)(nil)
)

type CreateMaintenanceWindowState struct {
	End                 time.Time
	SuppressionType     string
	MaintenanceWindowId *string
	ExperimentUri       *string
	ExecutionUri        *string
	ExperimentKey       *string
	ExecutionId         *int
}

func NewMaintenanceAction() action_kit_sdk.Action[CreateMaintenanceWindowState] {
	return &CreateMaintenanceWindowAction{}
}
func (m *CreateMaintenanceWindowAction) NewEmptyState() CreateMaintenanceWindowState {
	return CreateMaintenanceWindowState{}
}

const DetectProblemsAndAlert = "DETECT_PROBLEMS_AND_ALERT"
const DetectProblemsDontAlert = "DETECT_PROBLEMS_DONT_ALERT"
const DontDetectProblems = "DONT_DETECT_PROBLEMS"

func (m *CreateMaintenanceWindowAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          MaintenanceActionId,
		Label:       "Create Maintenance Window",
		Description: "Create a Maintenance Window for a given duration.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(maintenanceActionIcon),
		Category:    extutil.Ptr("monitoring"),
		Kind:        action_kit_api.Other,
		TimeControl: action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long should the maintenance window last?"),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("30s"),
				Order:        extutil.Ptr(0),
				Required:     extutil.Ptr(true),
			},
			{
				Name:         "suppressionType",
				Label:        "Problem detection and alerting",
				Type:         action_kit_api.String,
				DefaultValue: extutil.Ptr(DetectProblemsAndAlert),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Value: DetectProblemsAndAlert,
						Label: "Detect problems and alert",
					},
					action_kit_api.ExplicitParameterOption{
						Value: DetectProblemsDontAlert,
						Label: "Detect problems but don't alert",
					},
					action_kit_api.ExplicitParameterOption{
						Value: DontDetectProblems,
						Label: "Disable problem detection during maintenance",
					},
				}),
				Order:    extutil.Ptr(1),
				Required: extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (m *CreateMaintenanceWindowAction) Prepare(_ context.Context, state *CreateMaintenanceWindowState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	duration := request.Config["duration"].(float64)
	end := time.Now().UTC().Add(time.Millisecond * time.Duration(duration))

	state.End = end
	state.SuppressionType = request.Config["suppressionType"].(string)
	state.ExperimentUri = request.ExecutionContext.ExperimentUri
	state.ExecutionUri = request.ExecutionContext.ExecutionUri
	state.ExperimentKey = request.ExecutionContext.ExperimentKey
	state.ExecutionId = request.ExecutionContext.ExecutionId
	return nil, nil
}

func (m *CreateMaintenanceWindowAction) Start(ctx context.Context, state *CreateMaintenanceWindowState) (*action_kit_api.StartResult, error) {
	return CreateMaintenanceWindow(ctx, state, &config.Config)
}

func (m *CreateMaintenanceWindowAction) Stop(ctx context.Context, state *CreateMaintenanceWindowState) (*action_kit_api.StopResult, error) {
	return DeleteMaintenanceWindow(ctx, state, &config.Config)
}

type MaintenanceWindowApi interface {
	CreateMaintenanceWindow(ctx context.Context, maintenanceWindow types.CreateMaintenanceWindowRequest) (*string, *http.Response, error)
	DeleteMaintenanceWindow(ctx context.Context, maintenanceWindowId string) (*http.Response, error)
}

func CreateMaintenanceWindow(ctx context.Context, state *CreateMaintenanceWindowState, api MaintenanceWindowApi) (*action_kit_api.StartResult, error) {
	name := "Steadybit"
	if state.ExperimentKey != nil && state.ExecutionId != nil {
		name = fmt.Sprintf("Steadybit %s - %d", *state.ExperimentKey, *state.ExecutionId)
	}

	description := ""
	if state.ExecutionUri != nil {
		description = description + fmt.Sprintf("\nExperiment: %s", *state.ExperimentUri)
	}
	if state.ExecutionUri != nil {
		description = description + fmt.Sprintf("\nExecution: %s", *state.ExecutionUri)
	}

	createRequest := types.CreateMaintenanceWindowRequest{
		SchemaId: "builtin:alerting.maintenance-window",
		Scope:    "environment",
		Value: types.MaintenanceWindow{
			Enabled: true,
			GeneralProperties: types.MaintenanceWindowGeneralProperties{
				Name:                             name,
				Description:                      description,
				MaintenanceType:                  "PLANNED",
				Suppression:                      state.SuppressionType,
				DisableSyntheticMonitorExecution: false,
			},
			Schedule: types.MaintenanceWindowSchedule{
				ScheduleType: "ONCE",
				OnceRecurrence: types.MaintenanceWindowScheduleOnceRecurrence{
					StartTime: time.Now().UTC().Format("2006-01-02T15:04:05"),
					EndTime:   state.End.Format("2006-01-02T15:04:05"),
					TimeZone:  "UTC",
				},
			},
		},
	}

	windowId, _, err := api.CreateMaintenanceWindow(ctx, createRequest)
	if err != nil {
		return nil, extension_kit.ToError("Failed to create maintenance windows.", err)
	}

	state.MaintenanceWindowId = windowId

	return &action_kit_api.StartResult{
		Messages: &action_kit_api.Messages{
			action_kit_api.Message{Level: extutil.Ptr(action_kit_api.Info), Message: fmt.Sprintf("Maintenance window created. (id %s)", *state.MaintenanceWindowId)},
		},
	}, nil
}

func DeleteMaintenanceWindow(ctx context.Context, state *CreateMaintenanceWindowState, api MaintenanceWindowApi) (*action_kit_api.StopResult, error) {
	if state.MaintenanceWindowId == nil {
		return nil, nil
	}

	resp, err := api.DeleteMaintenanceWindow(ctx, *state.MaintenanceWindowId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to delete maintenace window (id %s). Full response: %v", *state.MaintenanceWindowId, resp), err)
	}

	return &action_kit_api.StopResult{
		Messages: &action_kit_api.Messages{
			action_kit_api.Message{Level: extutil.Ptr(action_kit_api.Info), Message: fmt.Sprintf("Maintenance window deleted. (id %s)", *state.MaintenanceWindowId)},
		},
	}, nil
}
