// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

func TestCheckAndroid_ProcessAlive_NoCrash(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof com.test.app",
		[]byte("12345"),
		nil,
	)
	mock.On(
		"adb logcat -d -s AndroidRuntime:E",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb logcat -d",
		[]byte("normal log line\n"),
		nil,
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test.app"),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
	assert.False(t, result.HasCrash)
	assert.False(t, result.HasANR)
	assert.Empty(t, result.LogEntries)
}

func TestCheckAndroid_ProcessDead_CrashDetected(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof com.test.app",
		[]byte(""),
		fmt.Errorf("exit code 1"),
	)
	mock.On(
		"adb logcat -d -s AndroidRuntime:E",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb logcat -d",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb shell screencap -p /sdcard/helixqa-check.png",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb pull",
		[]byte(""),
		nil,
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test.app"),
		WithEvidenceDir(t.TempDir()),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	assert.False(t, result.ProcessAlive)
	assert.True(t, result.HasCrash)
	assert.Contains(t, result.LogEntries[0],
		"process not alive")
}

func TestCheckAndroid_FatalException(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof com.test.app",
		[]byte("12345"),
		nil,
	)
	crashLog := "E/AndroidRuntime: FATAL EXCEPTION: main\n" +
		"java.lang.NullPointerException\n" +
		"at com.test.app.MainActivity.onCreate\n"
	mock.On(
		"adb logcat -d -s AndroidRuntime:E",
		[]byte(crashLog),
		nil,
	)
	mock.On(
		"adb logcat -d",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb shell screencap",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb pull",
		[]byte(""),
		nil,
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test.app"),
		WithEvidenceDir(t.TempDir()),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	assert.True(t, result.HasCrash)
	assert.NotEmpty(t, result.StackTrace)
	assert.True(t, len(result.LogEntries) > 0)
}

func TestCheckAndroid_ANRDetected(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof com.test.app",
		[]byte("12345"),
		nil,
	)
	mock.On(
		"adb logcat -d -s AndroidRuntime:E",
		[]byte(""),
		nil,
	)
	anrLog := "ActivityManager: ANR in com.test.app " +
		"(com.test.app/.MainActivity)\n"
	mock.On(
		"adb logcat -d",
		[]byte(anrLog),
		nil,
	)
	mock.On(
		"adb shell screencap",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb pull",
		[]byte(""),
		nil,
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test.app"),
		WithEvidenceDir(t.TempDir()),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	assert.True(t, result.HasANR)
	assert.True(t, len(result.LogEntries) > 0)
}

func TestCheckAndroid_WithDevice(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s emulator-5554 shell pidof com.test",
		[]byte("12345"),
		nil,
	)
	mock.On(
		"adb -s emulator-5554 logcat -d -s AndroidRuntime:E",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb -s emulator-5554 logcat -d",
		[]byte(""),
		nil,
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithDevice("emulator-5554"),
		WithPackageName("com.test"),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
}

func TestCheckAndroid_ProcessCheckError(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof com.test",
		nil,
		fmt.Errorf("adb not found"),
	)
	mock.On(
		"adb logcat -d -s AndroidRuntime:E",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb logcat -d",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb shell screencap",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb pull",
		[]byte(""),
		nil,
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test"),
		WithEvidenceDir(t.TempDir()),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	// pidof returning error means process not found.
	assert.False(t, result.ProcessAlive)
	assert.True(t, result.HasCrash)
}

func TestCheckAndroid_CrashLogError(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof com.test",
		[]byte("12345"),
		nil,
	)
	mock.On(
		"adb logcat -d -s AndroidRuntime:E",
		nil,
		fmt.Errorf("logcat error"),
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test"),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	assert.Contains(t, result.Error, "failed to read crash logs")
}

func TestCheckAndroid_ANRLogError(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof com.test",
		[]byte("12345"),
		nil,
	)
	mock.On(
		"adb logcat -d -s AndroidRuntime:E",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb logcat -d",
		nil,
		fmt.Errorf("logcat error"),
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test"),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	assert.Contains(t, result.Error, "failed to read ANR logs")
}

func TestAdbArgs_WithDevice(t *testing.T) {
	d := &Detector{device: "emulator-5554"}
	args := d.adbArgs("shell", "ls")
	assert.Equal(t, []string{
		"-s", "emulator-5554", "shell", "ls",
	}, args)
}

func TestAdbArgs_WithoutDevice(t *testing.T) {
	d := &Detector{}
	args := d.adbArgs("shell", "ls")
	assert.Equal(t, []string{"shell", "ls"}, args)
}

func TestCheckAndroid_CrashAndANR(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof com.test",
		[]byte("12345"),
		nil,
	)
	crashLog := "FATAL EXCEPTION in com.test\n" +
		"Exception at com.test.Main\n"
	mock.On(
		"adb logcat -d -s AndroidRuntime:E",
		[]byte(crashLog),
		nil,
	)
	anrLog := "ANR in com.test (com.test/.Main)\n"
	mock.On(
		"adb logcat -d",
		[]byte(anrLog),
		nil,
	)
	mock.On(
		"adb shell screencap",
		[]byte(""),
		nil,
	)
	mock.On(
		"adb pull",
		[]byte(""),
		nil,
	)

	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test"),
		WithEvidenceDir(t.TempDir()),
	)

	result, err := d.checkAndroid(context.Background())
	require.NoError(t, err)
	assert.True(t, result.HasCrash)
	assert.True(t, result.HasANR)
}
