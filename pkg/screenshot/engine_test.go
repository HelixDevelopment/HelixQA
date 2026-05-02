package screenshot

import (
	"context"
	"testing"

	"digital.vasic.helixqa/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestLinuxEngine_Name(t *testing.T) {
	eng := NewLinuxEngine()
	name := eng.Name()
	assert.True(t, name == "linux-unavailable" || name == "linux-xwd" || name == "linux-import" || name == "linux-grim")
}

func TestWebEngine_Name(t *testing.T) {
	eng := NewWebEngine("")
	assert.Equal(t, "web-playwright", eng.Name())
}

func TestIOSEngine_Name(t *testing.T) {
	eng := NewIOSEngine("test-device")
	assert.Equal(t, "ios-xcrun", eng.Name())
}

func TestAndroidEngine_Name(t *testing.T) {
	eng := NewAndroidEngine("test-device")
	assert.Equal(t, "android-adb", eng.Name())
}

func TestCLIEngine_Name(t *testing.T) {
	eng := NewCLIEngine()
	assert.Equal(t, "cli-terminal", eng.Name())
}

func TestTUIEngine_Name(t *testing.T) {
	eng := NewTUIEngine()
	assert.Equal(t, "tui-terminal", eng.Name())
}

func TestMacOSEngine_Name(t *testing.T) {
	eng := NewMacOSEngine()
	assert.Equal(t, "macos-screencapture", eng.Name())
}

func TestWindowsEngine_Name(t *testing.T) {
	eng := NewWindowsEngine()
	assert.Equal(t, "windows-powershell", eng.Name())
}

func TestResult_MetadataJSON(t *testing.T) {
	result := &Result{
		Data:     []byte("test"),
		Format:   "png",
		Platform: config.PlatformWeb,
		Engine:   "web-playwright",
	}
	json, err := result.MetadataJSON()
	assert.NoError(t, err)
	assert.Contains(t, json, "web-playwright")
	assert.Contains(t, json, "web")
}

func TestResult_MetadataJSON_Nil(t *testing.T) {
	var result *Result
	json, err := result.MetadataJSON()
	assert.Error(t, err)
	assert.Empty(t, json)
}

func TestNewManager_NilStorage(t *testing.T) {
	mgr := NewManager(nil)
	assert.NotNil(t, mgr)
}

func TestEnginePlatformMapping(t *testing.T) {
	tests := []struct {
		name   string
		engine Engine
	}{
		{"linux", NewLinuxEngine()},
		{"web", NewWebEngine("")},
		{"ios", NewIOSEngine("")},
		{"android", NewAndroidEngine("")},
		{"cli", NewCLIEngine()},
		{"tui", NewTUIEngine()},
		{"macos", NewMacOSEngine()},
		{"windows", NewWindowsEngine()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			supported := tt.engine.Supported(ctx)
			_ = supported // just exercise the method
			assert.NotEmpty(t, tt.engine.Name())
		})
	}
}

func TestPlatformMapping(t *testing.T) {
	assert.Equal(t, "linux", string(config.PlatformLinux))
	assert.Equal(t, "web", string(config.PlatformWeb))
	assert.Equal(t, "ios", string(config.PlatformIOS))
	assert.Equal(t, "android", string(config.PlatformAndroid))
	assert.Equal(t, "tui", string(config.PlatformTUI))
	assert.Equal(t, "desktop", string(config.PlatformDesktop))
}
