package extmaintenance

import (
	"context"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-dynatrace/types"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"time"
)

type dynatraceApiMock struct {
	mock.Mock
}

func (m *dynatraceApiMock) CreateMaintenanceWindow(ctx context.Context, maintenanceWindow types.CreateMaintenanceWindowRequest) (*string, *http.Response, error) {
	args := m.Called(ctx, maintenanceWindow)
	return args.Get(0).(*string), args.Get(1).(*http.Response), args.Error(2)
}

func (m *dynatraceApiMock) DeleteMaintenanceWindow(ctx context.Context, maintenanceWindowId string) (*http.Response, error) {
	args := m.Called(ctx, maintenanceWindowId)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestCreateMaintenanceWindowPrepareExtractsState(t *testing.T) {
	// Given
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]any{
			"duration":        1000 * 60,
			"suppressionType": "DETECT_PROBLEMS_DONT_ALERT",
		},
		ExecutionContext: new(action_kit_api.ExecutionContext{
			ExperimentUri: new("<uri-to-experiment>"),
			ExecutionUri:  new("<uri-to-execution>"),
			ExperimentKey: new("<experiment-key>"),
			ExecutionId:   new(4711),
		}),
	})
	action := CreateMaintenanceWindowAction{}
	state := action.NewEmptyState()

	// When
	result, err := action.Prepare(context.TODO(), &state, request)

	// Then
	require.Nil(t, result)
	require.Nil(t, err)
	require.Equal(t, "<uri-to-experiment>", *state.ExperimentUri)
	require.Equal(t, "<uri-to-execution>", *state.ExecutionUri)
	require.Equal(t, 4711, *state.ExecutionId)
	require.Equal(t, "<experiment-key>", *state.ExperimentKey)
	require.True(t, state.End.After(time.Now()))
}

func TestCreateMaintenanceWindowStartSuccess(t *testing.T) {
	// Given
	mockedApi := new(dynatraceApiMock)
	mockedApi.On("CreateMaintenanceWindow", mock.Anything, mock.Anything, mock.Anything).Return(
		new("this-is-the-id"),
		new(http.Response{
			StatusCode: 200,
		}), nil).Once()

	action := CreateMaintenanceWindowAction{}
	state := action.NewEmptyState()
	state.End = time.Now().Add(time.Minute)
	state.SuppressionType = "DETECT_PROBLEMS_DONT_ALERT"
	state.ExecutionUri = new("<uri-to-execution>")
	state.ExperimentUri = new("<uri-to-experiment>")
	state.ExperimentKey = new("<experiment-key>")
	state.ExecutionId = new(4711)

	// When
	result, err := CreateMaintenanceWindow(context.Background(), &state, mockedApi)

	// Then
	require.Nil(t, err)
	require.Nil(t, result.State)
	require.Equal(t, "this-is-the-id", *state.MaintenanceWindowId)
	require.Equal(t, "Maintenance window created. (id this-is-the-id)", (*result.Messages)[0].Message)
}

func TestCreateMaintenanceWindowStopSuccess(t *testing.T) {
	// Given
	mockedApi := new(dynatraceApiMock)
	mockedApi.On("DeleteMaintenanceWindow", mock.Anything, mock.Anything).Return(new(http.Response{
		StatusCode: 200,
	}), nil).Once()

	action := CreateMaintenanceWindowAction{}
	state := action.NewEmptyState()
	state.End = time.Now().Add(time.Minute)
	state.SuppressionType = "DETECT_PROBLEMS_DONT_ALERT"
	state.ExecutionUri = new("<uri-to-execution>")
	state.ExperimentUri = new("<uri-to-experiment>")
	state.ExperimentKey = new("<experiment-key>")
	state.ExecutionId = new(4711)
	state.MaintenanceWindowId = new("this-is-the-id")

	// When
	result, err := DeleteMaintenanceWindow(context.Background(), &state, mockedApi)

	// Then
	require.Nil(t, err)
	require.Equal(t, "Maintenance window deleted. (id this-is-the-id)", (*result.Messages)[0].Message)
}
