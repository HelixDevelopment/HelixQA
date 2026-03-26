// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package video

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScrcpyRecorder_StartState(t *testing.T) {
	r := NewScrcpyRecorder("emulator-5554", "/tmp/out.mp4")
	assert.False(t, r.IsRecording(),
		"recorder must not be recording after construction")
}

func TestScrcpyRecorder_BuildCommand_Scrcpy(t *testing.T) {
	r := NewScrcpyRecorder(
		"emulator-5554",
		"/tmp/out.mp4",
		WithMethod(MethodScrcpy),
	)
	args := r.buildScrcpyArgs()

	assert.Contains(t, args, "--serial",
		"scrcpy args must contain --serial flag")
	assert.Contains(t, args, "emulator-5554",
		"scrcpy args must contain device serial value")
	assert.Contains(t, args, "--record",
		"scrcpy args must contain --record flag")
	assert.Contains(t, args, "/tmp/out.mp4",
		"scrcpy args must contain output path")
}

func TestScrcpyRecorder_BuildCommand_ADB(t *testing.T) {
	r := NewScrcpyRecorder(
		"emulator-5554",
		"/tmp/out.mp4",
		WithMethod(MethodADBScreenrecord),
	)
	args := r.buildADBArgs()

	assert.Contains(t, args, "-s",
		"adb args must contain -s flag")
	assert.Contains(t, args, "emulator-5554",
		"adb args must contain device serial value")
	assert.Contains(t, args, "screenrecord",
		"adb args must contain screenrecord subcommand")
}

func TestScrcpyRecorder_MethodSelection(t *testing.T) {
	tests := []struct {
		name           string
		method         RecordMethod
		wantMethod     RecordMethod
		wantMethodName string
	}{
		{
			name:           "explicit scrcpy",
			method:         MethodScrcpy,
			wantMethod:     MethodScrcpy,
			wantMethodName: "MethodScrcpy",
		},
		{
			name:           "explicit adb screenrecord",
			method:         MethodADBScreenrecord,
			wantMethod:     MethodADBScreenrecord,
			wantMethodName: "MethodADBScreenrecord",
		},
		{
			name:           "screenshot assembly",
			method:         MethodScreenshotAssembly,
			wantMethod:     MethodScreenshotAssembly,
			wantMethodName: "MethodScreenshotAssembly",
		},
		{
			name:           "auto resolves to known method",
			method:         MethodAuto,
			wantMethodName: "MethodAuto",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := NewScrcpyRecorder(
				"device-123",
				"/tmp/rec.mp4",
				WithMethod(tc.method),
			)
			assert.Equal(t, tc.method, r.method,
				"method field must match configured value")

			if tc.method == MethodAuto {
				resolved := r.detectMethod()
				assert.True(
					t,
					resolved == MethodScrcpy ||
						resolved == MethodADBScreenrecord,
					"auto detection must resolve to scrcpy or adb",
				)
			}
		})
	}
}

func TestScrcpyRecorder_Accessors(t *testing.T) {
	r := NewScrcpyRecorder(
		"serial-xyz",
		"/recordings/test.mp4",
		WithBitRate(8_000_000),
		WithMaxDuration(60),
	)

	assert.Equal(t, "serial-xyz", r.Device())
	assert.Equal(t, "/recordings/test.mp4", r.OutputPath())
	assert.Equal(t, 8_000_000, r.bitRate)
	assert.Equal(t, 60, r.maxSecs)
	assert.Equal(t, 0, int(r.Duration()))
}

func TestScrcpyRecorder_StopWithoutStart(t *testing.T) {
	r := NewScrcpyRecorder("emulator-5554", "/tmp/out.mp4")
	err := r.Stop()
	assert.Error(t, err,
		"Stop without Start must return an error")
}
