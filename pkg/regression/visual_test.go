// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package regression

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock vision provider ---

// mockVisionProv returns canned responses for Vision calls.
type mockVisionProv struct {
	response    string
	supportsVis bool
	visionErr   error
	callCount   atomic.Int32
}

func (m *mockVisionProv) Vision(
	_ context.Context, _ []byte, _ string,
) (*VisionResponse, error) {
	m.callCount.Add(1)
	if m.visionErr != nil {
		return nil, m.visionErr
	}
	return &VisionResponse{Content: m.response}, nil
}

func (m *mockVisionProv) SupportsVision() bool {
	return m.supportsVis
}

// varyingVisionProv cycles through different responses.
type varyingVisionProv struct {
	responses []string
	index     atomic.Int32
}

func (v *varyingVisionProv) Vision(
	_ context.Context, _ []byte, _ string,
) (*VisionResponse, error) {
	idx := int(v.index.Add(1)) - 1
	if idx >= len(v.responses) {
		idx = len(v.responses) - 1
	}
	return &VisionResponse{
		Content: v.responses[idx],
	}, nil
}

func (v *varyingVisionProv) SupportsVision() bool {
	return true
}

// --- helper ---

func fakeScreenshot() []byte {
	return []byte("fake-png-data")
}

// --- NewVisualRegression tests ---

func TestNewVisualRegression_Defaults(t *testing.T) {
	prov := &mockVisionProv{supportsVis: true}
	vr := NewVisualRegression(prov)
	assert.Equal(t, 1, vr.concurrency)
	assert.NotNil(t, vr.provider)
}

func TestNewVisualRegression_WithConcurrency(t *testing.T) {
	prov := &mockVisionProv{supportsVis: true}
	vr := NewVisualRegression(prov, WithConcurrency(4))
	assert.Equal(t, 4, vr.concurrency)
}

func TestNewVisualRegression_InvalidConcurrency(
	t *testing.T,
) {
	prov := &mockVisionProv{supportsVis: true}
	vr := NewVisualRegression(prov, WithConcurrency(0))
	assert.Equal(t, 1, vr.concurrency)

	vr = NewVisualRegression(prov, WithConcurrency(-1))
	assert.Equal(t, 1, vr.concurrency)
}

// --- Compare tests ---

func TestCompare_SingleScreenshot(t *testing.T) {
	prov := &mockVisionProv{supportsVis: true}
	vr := NewVisualRegression(prov)

	result, err := vr.Compare(context.Background(),
		[]DeviceScreenshot{
			{
				Device:     "phone-1",
				Platform:   "android",
				Screenshot: fakeScreenshot(),
				Step:       1,
			},
		},
	)

	require.NoError(t, err)
	assert.True(t, result.Consistent)
	assert.Equal(t, 1, result.Step)
	assert.Len(t, result.Devices, 1)
	assert.Empty(t, result.Differences)
}

func TestCompare_EmptyScreenshots(t *testing.T) {
	prov := &mockVisionProv{supportsVis: true}
	vr := NewVisualRegression(prov)

	result, err := vr.Compare(
		context.Background(), nil,
	)

	require.NoError(t, err)
	assert.True(t, result.Consistent)
}

func TestCompare_TwoDevices_NoDifferences(t *testing.T) {
	prov := &mockVisionProv{
		supportsVis: true,
		response: `{"different": false, ` +
			`"description": "no differences", ` +
			`"severity": "info"}`,
	}
	vr := NewVisualRegression(prov)

	result, err := vr.Compare(context.Background(),
		[]DeviceScreenshot{
			{
				Device:     "phone-1",
				Platform:   "android",
				Screenshot: fakeScreenshot(),
				Step:       3,
			},
			{
				Device:     "tablet-1",
				Platform:   "android",
				Screenshot: fakeScreenshot(),
				Step:       3,
			},
		},
	)

	require.NoError(t, err)
	assert.True(t, result.Consistent)
	assert.Equal(t, 3, result.Step)
	assert.Len(t, result.Devices, 2)
	assert.Empty(t, result.Differences)
	assert.Equal(t, 1, result.ComparisonsMade)
}

func TestCompare_TwoDevices_WithDifference(t *testing.T) {
	prov := &mockVisionProv{
		supportsVis: true,
		response: `{"different": true, ` +
			`"description": "Button is truncated on phone", ` +
			`"severity": "warning"}`,
	}
	vr := NewVisualRegression(prov)

	result, err := vr.Compare(context.Background(),
		[]DeviceScreenshot{
			{
				Device:     "phone-1",
				Platform:   "android",
				Screenshot: fakeScreenshot(),
				Step:       1,
			},
			{
				Device:     "tv-1",
				Platform:   "androidtv",
				Screenshot: fakeScreenshot(),
				Step:       1,
			},
		},
	)

	require.NoError(t, err)
	assert.False(t, result.Consistent)
	assert.Len(t, result.Differences, 1)
	assert.Equal(t, "phone-1", result.Differences[0].DeviceA)
	assert.Equal(t, "tv-1", result.Differences[0].DeviceB)
	assert.Equal(t,
		SeverityWarning, result.Differences[0].Severity,
	)
	assert.Contains(t,
		result.Differences[0].Description, "truncated",
	)
}

func TestCompare_ThreeDevices_PairwiseComparisons(
	t *testing.T,
) {
	prov := &mockVisionProv{
		supportsVis: true,
		response: `{"different": true, ` +
			`"description": "layout shift", ` +
			`"severity": "critical"}`,
	}
	vr := NewVisualRegression(prov)

	result, err := vr.Compare(context.Background(),
		[]DeviceScreenshot{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d3", Platform: "androidtv",
				Screenshot: fakeScreenshot(), Step: 1,
			},
		},
	)

	require.NoError(t, err)
	assert.False(t, result.Consistent)
	// 3 devices = C(3,2) = 3 pairwise comparisons.
	assert.Equal(t, 3, result.ComparisonsMade)
	assert.Len(t, result.Differences, 3)
}

func TestCompare_VisionNotSupported(t *testing.T) {
	prov := &mockVisionProv{supportsVis: false}
	vr := NewVisualRegression(prov)

	_, err := vr.Compare(context.Background(),
		[]DeviceScreenshot{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(),
		"does not support vision")
}

func TestCompare_VisionError(t *testing.T) {
	prov := &mockVisionProv{
		supportsVis: true,
		visionErr:   fmt.Errorf("rate limited"),
	}
	vr := NewVisualRegression(prov)

	result, err := vr.Compare(context.Background(),
		[]DeviceScreenshot{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
		},
	)

	// Vision errors are skipped; result is consistent
	// because no differences could be confirmed.
	require.NoError(t, err)
	assert.True(t, result.Consistent)
	assert.Empty(t, result.Differences)
}

func TestCompare_ValidationError_EmptyDevice(t *testing.T) {
	prov := &mockVisionProv{supportsVis: true}
	vr := NewVisualRegression(prov)

	_, err := vr.Compare(context.Background(),
		[]DeviceScreenshot{
			{
				Device: "", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(),
		"device identifier is required")
}

func TestCompare_ValidationError_EmptyScreenshot(
	t *testing.T,
) {
	prov := &mockVisionProv{supportsVis: true}
	vr := NewVisualRegression(prov)

	_, err := vr.Compare(context.Background(),
		[]DeviceScreenshot{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: nil, Step: 1,
			},
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "screenshot data is empty")
}

func TestCompare_ContextCanceled(t *testing.T) {
	prov := &mockVisionProv{
		supportsVis: true,
		response:    `{"different": false}`,
	}
	vr := NewVisualRegression(prov)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := vr.Compare(ctx,
		[]DeviceScreenshot{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
		},
	)

	assert.Error(t, err)
}

func TestCompare_ConcurrentComparisons(t *testing.T) {
	prov := &mockVisionProv{
		supportsVis: true,
		response: `{"different": true, ` +
			`"description": "diff", "severity": "info"}`,
	}
	vr := NewVisualRegression(prov, WithConcurrency(3))

	screenshots := make([]DeviceScreenshot, 4)
	for i := range screenshots {
		screenshots[i] = DeviceScreenshot{
			Device:     fmt.Sprintf("d%d", i+1),
			Platform:   "android",
			Screenshot: fakeScreenshot(),
			Step:       1,
		}
	}

	result, err := vr.Compare(
		context.Background(), screenshots,
	)

	require.NoError(t, err)
	// C(4,2) = 6 pairwise comparisons.
	assert.Equal(t, 6, result.ComparisonsMade)
	assert.Len(t, result.Differences, 6)
}

// --- CompareMultipleSteps tests ---

func TestCompareMultipleSteps_AllConsistent(t *testing.T) {
	prov := &mockVisionProv{
		supportsVis: true,
		response: `{"different": false, ` +
			`"description": "same", "severity": "info"}`,
	}
	vr := NewVisualRegression(prov)

	steps := [][]DeviceScreenshot{
		{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
		},
		{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 2,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 2,
			},
		},
	}

	results, err := vr.CompareMultipleSteps(
		context.Background(), steps,
	)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, results[0].Consistent)
	assert.True(t, results[1].Consistent)
}

func TestCompareMultipleSteps_MixedResults(t *testing.T) {
	prov := &varyingVisionProv{
		responses: []string{
			`{"different": false}`,
			`{"different": true, "description": "gap", "severity": "warning"}`,
		},
	}
	vr := NewVisualRegression(prov)

	steps := [][]DeviceScreenshot{
		{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
		},
		{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 2,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 2,
			},
		},
	}

	results, err := vr.CompareMultipleSteps(
		context.Background(), steps,
	)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, results[0].Consistent)
	assert.False(t, results[1].Consistent)
}

func TestCompareMultipleSteps_ContextCanceled(t *testing.T) {
	prov := &mockVisionProv{
		supportsVis: true,
		response:    `{"different": false}`,
	}
	vr := NewVisualRegression(prov)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	steps := [][]DeviceScreenshot{
		{
			{
				Device: "d1", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
			{
				Device: "d2", Platform: "android",
				Screenshot: fakeScreenshot(), Step: 1,
			},
		},
	}

	_, err := vr.CompareMultipleSteps(ctx, steps)
	assert.Error(t, err)
}

// --- Utility function tests ---

func TestConsistencyRate_AllConsistent(t *testing.T) {
	results := []*RegressionResult{
		{Consistent: true},
		{Consistent: true},
		{Consistent: true},
	}
	rate := ConsistencyRate(results)
	assert.InDelta(t, 1.0, rate, 0.001)
}

func TestConsistencyRate_NoneConsistent(t *testing.T) {
	results := []*RegressionResult{
		{Consistent: false},
		{Consistent: false},
	}
	rate := ConsistencyRate(results)
	assert.InDelta(t, 0.0, rate, 0.001)
}

func TestConsistencyRate_Mixed(t *testing.T) {
	results := []*RegressionResult{
		{Consistent: true},
		{Consistent: false},
		{Consistent: true},
		{Consistent: false},
	}
	rate := ConsistencyRate(results)
	assert.InDelta(t, 0.5, rate, 0.001)
}

func TestConsistencyRate_Empty(t *testing.T) {
	rate := ConsistencyRate(nil)
	assert.InDelta(t, 1.0, rate, 0.001)
}

func TestTotalDifferences(t *testing.T) {
	results := []*RegressionResult{
		{
			Differences: []VisualDifference{
				{Description: "a"},
				{Description: "b"},
			},
		},
		{Differences: nil},
		{
			Differences: []VisualDifference{
				{Description: "c"},
			},
		},
	}
	assert.Equal(t, 3, TotalDifferences(results))
}

func TestTotalDifferences_Empty(t *testing.T) {
	assert.Equal(t, 0, TotalDifferences(nil))
}

// --- RegressionResult.CriticalCount tests ---

func TestRegressionResult_CriticalCount(t *testing.T) {
	r := &RegressionResult{
		Differences: []VisualDifference{
			{Severity: SeverityCritical},
			{Severity: SeverityWarning},
			{Severity: SeverityCritical},
			{Severity: SeverityInfo},
		},
	}
	assert.Equal(t, 2, r.CriticalCount())
}

func TestRegressionResult_CriticalCount_None(t *testing.T) {
	r := &RegressionResult{
		Differences: []VisualDifference{
			{Severity: SeverityWarning},
			{Severity: SeverityInfo},
		},
	}
	assert.Equal(t, 0, r.CriticalCount())
}

// --- ValidSeverity tests ---

func TestValidSeverity(t *testing.T) {
	assert.True(t, ValidSeverity("critical"))
	assert.True(t, ValidSeverity("warning"))
	assert.True(t, ValidSeverity("info"))
	assert.False(t, ValidSeverity("high"))
	assert.False(t, ValidSeverity(""))
	assert.False(t, ValidSeverity("CRITICAL"))
}

// --- DeviceScreenshot.Validate tests ---

func TestDeviceScreenshot_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ss      DeviceScreenshot
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid",
			ss: DeviceScreenshot{
				Device:     "phone-1",
				Screenshot: fakeScreenshot(),
			},
			wantErr: false,
		},
		{
			name: "missing device",
			ss: DeviceScreenshot{
				Screenshot: fakeScreenshot(),
			},
			wantErr: true,
			errMsg:  "device identifier",
		},
		{
			name: "empty screenshot",
			ss: DeviceScreenshot{
				Device: "phone-1",
			},
			wantErr: true,
			errMsg:  "screenshot data is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ss.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- parseComparisonResponse tests ---

func TestParseComparisonResponse_Different(t *testing.T) {
	content := `{"different": true, "description": "layout mismatch", "severity": "critical"}`
	diff := parseComparisonResponse(content, "d1", "d2")

	require.NotNil(t, diff)
	assert.Equal(t, "d1", diff.DeviceA)
	assert.Equal(t, "d2", diff.DeviceB)
	assert.Equal(t, "layout mismatch", diff.Description)
	assert.Equal(t, SeverityCritical, diff.Severity)
}

func TestParseComparisonResponse_NotDifferent(t *testing.T) {
	content := `{"different": false, "description": "same", "severity": "info"}`
	diff := parseComparisonResponse(content, "d1", "d2")
	assert.Nil(t, diff)
}

func TestParseComparisonResponse_InvalidJSON(t *testing.T) {
	diff := parseComparisonResponse(
		"not json at all", "d1", "d2",
	)
	assert.Nil(t, diff)
}

func TestParseComparisonResponse_EmbeddedJSON(t *testing.T) {
	content := `Here is my analysis: {"different": true, "description": "color shift", "severity": "warning"} that's it.`
	diff := parseComparisonResponse(content, "d1", "d2")

	require.NotNil(t, diff)
	assert.Equal(t, "color shift", diff.Description)
	assert.Equal(t, SeverityWarning, diff.Severity)
}

func TestParseComparisonResponse_InvalidSeverity(
	t *testing.T,
) {
	content := `{"different": true, "description": "issue", "severity": "extreme"}`
	diff := parseComparisonResponse(content, "d1", "d2")

	require.NotNil(t, diff)
	// Invalid severity should default to warning.
	assert.Equal(t, SeverityWarning, diff.Severity)
}

func TestParseComparisonResponse_EmptyContent(t *testing.T) {
	diff := parseComparisonResponse("", "d1", "d2")
	assert.Nil(t, diff)
}
