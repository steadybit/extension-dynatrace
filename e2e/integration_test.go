// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	"github.com/steadybit/extension-dynatrace/extmaintenance"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestWithMinikube(t *testing.T) {
	extlogging.InitZeroLog()
	server := createMockDynatraceServer()
	defer server.Close()
	split := strings.SplitAfter(server.URL, ":")
	port := split[len(split)-1]

	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-dynatrace",
		Port: 8090,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=debug",
				"--set", "dynatrace.apiToken=api-token-123",
				"--set", "dynatrace.apiBaseUrl=http://host.minikube.internal:" + port,
			}
		},
	}

	e2e.WithDefaultMinikube(t, &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "create maintenance window",
			Test: testCreateMaintenanceWindow,
		},
	})
}

func testCreateMaintenanceWindow(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	defer func() { Requests = []string{} }()

	config := struct {
		Duration        int    `json:"duration"`
		SuppressionType string `json:"suppressionType"`
	}{Duration: 1000, SuppressionType: "DETECT_PROBLEMS_DONT_ALERT"}

	executionContext := &action_kit_api.ExecutionContext{}

	action, err := e.RunAction(extmaintenance.MaintenanceActionId, nil, config, executionContext)
	defer func() { _ = action.Cancel() }()
	require.NoError(t, err)
	err = action.Wait()
	require.NoError(t, err)
	require.Contains(t, Requests, "POST-/v2/settings/objects")
	require.Contains(t, Requests, "DELETE-/v2/settings/objects/MOCKED-MAINTENANCE-WINDOW-ID")
}
