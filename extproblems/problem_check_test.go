// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extproblems

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

type problemsApiMock struct {
	mock.Mock
}

func (m *problemsApiMock) GetProblems(ctx context.Context, from time.Time, entitySelector *string) ([]types.Problem, *http.Response, error) {
	args := m.Called(ctx, from, entitySelector)
	return args.Get(0).([]types.Problem), args.Get(1).(*http.Response), args.Error(2)
}

func TestPrepareDefaultsFailEarlyToTrue(t *testing.T) {
	// Given
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"duration":           1000 * 60,
			"condition":          conditionNoProblems,
			"conditionCheckMode": conditionCheckModeAllTheTime,
		},
	})
	action := ProblemCheckAction{}
	state := action.NewEmptyState()

	// When
	result, err := action.Prepare(context.TODO(), &state, request)

	// Then
	require.Nil(t, result)
	require.Nil(t, err)
	require.True(t, state.FailEarly) // defaults to true when not provided (non-breaking for old experiments)
}

func TestPrepareExtractsFailEarlyFalse(t *testing.T) {
	// Given
	request := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"duration":           1000 * 60,
			"condition":          conditionNoProblems,
			"conditionCheckMode": conditionCheckModeAllTheTime,
			"failEarly":          false,
		},
	})
	action := ProblemCheckAction{}
	state := action.NewEmptyState()

	// When
	result, err := action.Prepare(context.TODO(), &state, request)

	// Then
	require.Nil(t, result)
	require.Nil(t, err)
	require.False(t, state.FailEarly)
}

func TestAllTheTimeFailEarly(t *testing.T) {
	// Given - a problem exists while "No problem expected", time not yet up
	mockedApi := new(problemsApiMock)
	mockedApi.On("GetProblems", mock.Anything, mock.Anything, mock.Anything).Return([]types.Problem{{}}, extutil.Ptr(http.Response{StatusCode: 200}), nil)

	action := ProblemCheckAction{}
	state := action.NewEmptyState()
	state.Start = time.Now()
	state.End = time.Now().Add(time.Minute * 1) // time not yet up
	state.Condition = conditionNoProblems
	state.ConditionCheckMode = conditionCheckModeAllTheTime
	state.FailEarly = true

	// When
	result, err := ProblemCheckStatus(context.Background(), &state, mockedApi)

	// Then - fails immediately
	require.Nil(t, err)
	require.False(t, result.Completed)
	require.NotNil(t, result.Error)
	require.Equal(t, "No problem expected, but 1 problems found.", result.Error.Title)
}

func TestAllTheTimeFailAtEnd(t *testing.T) {
	// First call: deviation but time not up -> no error, deviation remembered
	mockedApi := new(problemsApiMock)
	mockedApi.On("GetProblems", mock.Anything, mock.Anything, mock.Anything).Return([]types.Problem{{}}, extutil.Ptr(http.Response{StatusCode: 200}), nil).Once()

	action := ProblemCheckAction{}
	state := action.NewEmptyState()
	state.Start = time.Now()
	state.End = time.Now().Add(time.Minute * 1) // time not yet up
	state.Condition = conditionNoProblems
	state.ConditionCheckMode = conditionCheckModeAllTheTime
	state.FailEarly = false

	result, err := ProblemCheckStatus(context.Background(), &state, mockedApi)
	require.Nil(t, err)
	require.False(t, result.Completed)
	require.Nil(t, result.Error) // does not fail early
	require.True(t, state.DeviationSeen)

	// Second call: problems gone but time is up and a deviation was seen -> fails at the end
	mockedApi.On("GetProblems", mock.Anything, mock.Anything, mock.Anything).Return([]types.Problem{}, extutil.Ptr(http.Response{StatusCode: 200}), nil).Once()
	state.End = time.Now().Add(time.Minute * -1) // time is up

	result, err = ProblemCheckStatus(context.Background(), &state, mockedApi)
	require.Nil(t, err)
	require.True(t, result.Completed)
	require.NotNil(t, result.Error)
	require.Equal(t, "No problem expected, but 1 problems found.", result.Error.Title)
}

func TestAllTheTimeFailAtEndSucceedsWhenNeverDeviated(t *testing.T) {
	// Given - no problems throughout while "No problem expected", time is up
	mockedApi := new(problemsApiMock)
	mockedApi.On("GetProblems", mock.Anything, mock.Anything, mock.Anything).Return([]types.Problem{}, extutil.Ptr(http.Response{StatusCode: 200}), nil)

	action := ProblemCheckAction{}
	state := action.NewEmptyState()
	state.Start = time.Now()
	state.End = time.Now().Add(time.Minute * -1) // time is up
	state.Condition = conditionNoProblems
	state.ConditionCheckMode = conditionCheckModeAllTheTime
	state.FailEarly = false

	// When
	result, err := ProblemCheckStatus(context.Background(), &state, mockedApi)

	// Then
	require.Nil(t, err)
	require.True(t, result.Completed)
	require.Nil(t, result.Error)
	require.False(t, state.DeviationSeen)
}
