// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extproblems

import (
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
)

func TestProblemCheckParameterGroups(t *testing.T) {
	desc := NewProblemCheckAction().Describe()

	var failEarly *action_kit_api.ActionParameter
	for i, p := range desc.Parameters {
		if p.Advanced != nil && *p.Advanced {
			assert.NotNil(t, p.Group, "advanced parameter %q must have a group", p.Name)
		}
		if p.Name == "failEarly" {
			failEarly = &desc.Parameters[i]
		}
	}

	if assert.NotNil(t, failEarly) {
		assert.False(t, failEarly.Advanced != nil && *failEarly.Advanced, "failEarly must not be advanced")
		assert.Nil(t, failEarly.Group, "failEarly must not be in a group")
	}

	order := func(p action_kit_api.ActionParameter) int {
		if p.Order == nil {
			return 0
		}
		return *p.Order
	}
	for _, grouped := range desc.Parameters {
		if grouped.Group == nil || *grouped.Group != action_kit_api.ParameterGroupAdditionalSettings {
			continue
		}
		for _, other := range desc.Parameters {
			if other.Group == nil || *other.Group != action_kit_api.ParameterGroupAdditionalSettings {
				assert.Greater(t, order(grouped), order(other), "%q must sort after %q", grouped.Name, other.Name)
			}
		}
	}
}
