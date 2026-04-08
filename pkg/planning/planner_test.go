// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/learning"
	"digital.vasic.helixqa/pkg/llm"
)

// mockLLM is a test double that satisfies llm.Provider.
type mockLLM struct {
	response string
}

func (m *mockLLM) Chat(
	_ context.Context,
	_ []llm.Message,
) (*llm.Response, error) {
	return &llm.Response{Content: m.response}, nil
}

func (m *mockLLM) Vision(
	_ context.Context,
	_ []byte,
	_ string,
) (*llm.Response, error) {
	return nil, nil
}

func (m *mockLLM) Name() string         { return "mock" }
func (m *mockLLM) SupportsVision() bool { return false }

// twoTestJSON is a valid JSON array of two PlannedTest objects returned
// by the mock LLM.
const twoTestJSON = `[
  {
    "id": "GEN-001",
    "name": "Login with valid credentials",
    "description": "Verifies that a user can log in with correct credentials",
    "category": "functional",
    "priority": 1,
    "platforms": ["web", "android"],
    "screen": "Login",
    "steps": ["Open login screen", "Enter valid email", "Enter valid password", "Tap login"],
    "expected": "User is redirected to the home screen"
  },
  {
    "id": "GEN-002",
    "name": "Login with invalid password",
    "description": "Verifies that an error message appears for wrong password",
    "category": "edge_case",
    "priority": 2,
    "platforms": ["web"],
    "screen": "Login",
    "steps": ["Open login screen", "Enter valid email", "Enter wrong password", "Tap login"],
    "expected": "Error message is displayed"
  }
]`

func TestTestPlanGenerator_Generate(t *testing.T) {
	mock := &mockLLM{response: twoTestJSON}
	gen := NewTestPlanGenerator(mock)

	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "Catalogizer"
	kb.AddScreen(learning.Screen{
		Name:     "Login",
		Platform: "web",
		Route:    "/login",
	})
	kb.AddEndpoint(learning.APIEndpoint{
		Method:  "POST",
		Path:    "/api/v1/auth/login",
		Handler: "AuthHandler",
	})

	plan, err := gen.Generate(
		context.Background(),
		kb,
		[]string{"web", "android"},
	)

	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.Equal(t, 2, plan.TotalTests,
		"plan should contain 2 tests")
	assert.Equal(t, 2, plan.NewTests,
		"all generated tests should be marked new")
	assert.Equal(t, 0, plan.ExistingTests,
		"no tests should be marked existing")
	assert.Equal(t, []string{"web", "android"}, plan.Platforms,
		"platforms should match the requested list")

	require.Len(t, plan.Tests, 2)

	t0 := plan.Tests[0]
	assert.Equal(t, "GEN-001", t0.ID)
	assert.Equal(t, "Login with valid credentials", t0.Name)
	assert.Equal(t, "functional", t0.Category)
	assert.Equal(t, 1, t0.Priority)
	assert.True(t, t0.IsNew, "parsed test should be IsNew=true")
	assert.False(t, t0.IsExisting,
		"parsed test should not be IsExisting")

	t1 := plan.Tests[1]
	assert.Equal(t, "GEN-002", t1.ID)
	assert.Equal(t, "edge_case", t1.Category)
	assert.True(t, t1.IsNew)
}

func TestTestPlanGenerator_EmptyKnowledge(t *testing.T) {
	mock := &mockLLM{response: "[]"}
	gen := NewTestPlanGenerator(mock)

	kb := learning.NewKnowledgeBase()

	plan, err := gen.Generate(context.Background(), kb, []string{"web"})

	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.Equal(t, 0, plan.TotalTests,
		"empty LLM response should produce 0 tests")
	assert.Equal(t, 0, plan.NewTests)
	assert.Equal(t, 0, plan.ExistingTests)
	assert.Empty(t, plan.Tests)
}

func TestTestPlanGenerator_MalformedLLMResponse(t *testing.T) {
	mock := &mockLLM{response: "not json"}
	gen := NewTestPlanGenerator(mock)

	kb := learning.NewKnowledgeBase()

	plan, err := gen.Generate(
		context.Background(),
		kb,
		[]string{"android"},
	)

	// Malformed LLM output must NOT cause an error — graceful
	// degradation to an empty plan is required.
	require.NoError(t, err,
		"malformed LLM response should degrade gracefully, not error")
	require.NotNil(t, plan)

	assert.Equal(t, 0, plan.TotalTests,
		"malformed response should produce 0 tests")
	assert.Empty(t, plan.Tests)
}

// duplicateTestJSON contains duplicate test names to verify deduplication.
const duplicateTestJSON = `[
  {
    "id": "GEN-001",
    "name": "Login Test",
    "description": "First login test",
    "category": "functional",
    "priority": 1,
    "platforms": ["web"],
    "screen": "Login",
    "steps": ["Step 1"],
    "expected": "Success"
  },
  {
    "id": "GEN-002",
    "name": "Login Test",
    "description": "Duplicate login test",
    "category": "functional",
    "priority": 2,
    "platforms": ["android"],
    "screen": "Login",
    "steps": ["Step 1"],
    "expected": "Success"
  },
  {
    "id": "GEN-003",
    "name": "login test",
    "description": "Case variant duplicate",
    "category": "edge_case",
    "priority": 3,
    "platforms": ["ios"],
    "screen": "Login",
    "steps": ["Step 1"],
    "expected": "Success"
  },
  {
    "id": "GEN-004",
    "name": "Unique Test",
    "description": "This one should be kept",
    "category": "functional",
    "priority": 1,
    "platforms": ["web"],
    "screen": "Home",
    "steps": ["Step 1"],
    "expected": "Success"
  }
]`

func TestTestPlanGenerator_Deduplication(t *testing.T) {
	mock := &mockLLM{response: duplicateTestJSON}
	gen := NewTestPlanGenerator(mock)

	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "Catalogizer"

	plan, err := gen.Generate(
		context.Background(),
		kb,
		[]string{"web", "android", "ios"},
	)

	require.NoError(t, err)
	require.NotNil(t, plan)

	// Should have only 2 tests: "Login Test" (first occurrence) and "Unique Test"
	assert.Equal(t, 2, plan.TotalTests,
		"duplicate tests should be removed")
	assert.Equal(t, 2, plan.NewTests)

	// Verify which tests were kept (first occurrence wins)
	names := make([]string, len(plan.Tests))
	for i, t := range plan.Tests {
		names[i] = t.Name
	}
	assert.Contains(t, names, "Login Test")
	assert.Contains(t, names, "Unique Test")
}
