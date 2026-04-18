package mobile

import "fmt"

// Capabilities carries the superset of W3C + Appium vendor capabilities
// Nexus ever sets. Adapters build one via NewAndroidCaps / NewIOSCaps /
// NewAndroidTVCaps helpers that fill the non-optional fields up-front.
type Capabilities struct {
	Platform   PlatformType
	DeviceName string
	OSVersion  string

	// Android-specific
	AppPackage  string
	AppActivity string
	UDID        string

	// iOS-specific
	BundleID          string
	XcodeOrgID        string
	XcodeSigningID    string
	WebDriverAgentURL string

	// Shared
	AutomationName       string
	AppPath              string
	NoReset              bool
	FullReset            bool
	NewCommandTimeoutSec int
	Locale               string
	Orientation          string

	// Free-form extras pass through verbatim; use sparingly.
	Extra map[string]any
}

// NewAndroidCaps returns a validated profile for UiAutomator2.
func NewAndroidCaps(device, pkg, activity string) Capabilities {
	return Capabilities{
		Platform:       PlatformAndroid,
		DeviceName:     device,
		AppPackage:     pkg,
		AppActivity:    activity,
		AutomationName: "UiAutomator2",
	}
}

// NewAndroidTVCaps returns a profile that still uses UiAutomator2 under
// the hood but tags the Platform differently so banks and adapters can
// filter Android TV runs out of phone runs.
func NewAndroidTVCaps(device, pkg, activity string) Capabilities {
	c := NewAndroidCaps(device, pkg, activity)
	c.Platform = PlatformAndroidTV
	return c
}

// NewIOSCaps returns a profile for XCUITest. The xcodeOrgID and
// xcodeSigningID are required for real devices; for simulators pass
// empty strings and the fields are omitted.
func NewIOSCaps(device, bundleID, udid, orgID, signingID string) Capabilities {
	return Capabilities{
		Platform:       PlatformIOS,
		DeviceName:     device,
		BundleID:       bundleID,
		UDID:           udid,
		XcodeOrgID:     orgID,
		XcodeSigningID: signingID,
		AutomationName: "XCUITest",
	}
}

// Validate reports obvious misconfigurations so adapters can fail fast.
// The rules mirror Appium's own validation without requiring a live hub
// connection.
func (c Capabilities) Validate() error {
	if c.Platform == "" {
		return fmt.Errorf("capabilities: platform is required")
	}
	if c.DeviceName == "" {
		return fmt.Errorf("capabilities: device name is required")
	}
	switch c.Platform {
	case PlatformAndroid, PlatformAndroidTV:
		if c.AppPackage == "" {
			return fmt.Errorf("android capabilities: appPackage is required")
		}
		if c.AppActivity == "" {
			return fmt.Errorf("android capabilities: appActivity is required")
		}
	case PlatformIOS:
		if c.BundleID == "" {
			return fmt.Errorf("ios capabilities: bundleId is required")
		}
		// Real-device deployments need team + signing id; simulators pass empty.
		if (c.XcodeOrgID == "") != (c.XcodeSigningID == "") {
			return fmt.Errorf("ios capabilities: xcodeOrgID and xcodeSigningID must be set together")
		}
	default:
		return fmt.Errorf("capabilities: unknown platform %q", c.Platform)
	}
	return nil
}

// toMap builds the Appium-vendor capabilities map sent in NewSession.
// Unknown extras pass through verbatim.
func (c Capabilities) toMap() map[string]any {
	m := map[string]any{
		"platformName":      platformName(c.Platform),
		"appium:deviceName": c.DeviceName,
	}
	if c.OSVersion != "" {
		m["appium:platformVersion"] = c.OSVersion
	}
	if c.AutomationName != "" {
		m["appium:automationName"] = c.AutomationName
	}
	if c.AppPath != "" {
		m["appium:app"] = c.AppPath
	}
	if c.UDID != "" {
		m["appium:udid"] = c.UDID
	}
	if c.NewCommandTimeoutSec > 0 {
		m["appium:newCommandTimeout"] = c.NewCommandTimeoutSec
	}
	if c.Locale != "" {
		m["appium:locale"] = c.Locale
	}
	if c.Orientation != "" {
		m["appium:orientation"] = c.Orientation
	}
	if c.NoReset {
		m["appium:noReset"] = true
	}
	if c.FullReset {
		m["appium:fullReset"] = true
	}
	switch c.Platform {
	case PlatformAndroid, PlatformAndroidTV:
		if c.AppPackage != "" {
			m["appium:appPackage"] = c.AppPackage
		}
		if c.AppActivity != "" {
			m["appium:appActivity"] = c.AppActivity
		}
	case PlatformIOS:
		if c.BundleID != "" {
			m["appium:bundleId"] = c.BundleID
		}
		if c.XcodeOrgID != "" {
			m["appium:xcodeOrgId"] = c.XcodeOrgID
		}
		if c.XcodeSigningID != "" {
			m["appium:xcodeSigningId"] = c.XcodeSigningID
		}
		if c.WebDriverAgentURL != "" {
			m["appium:webDriverAgentUrl"] = c.WebDriverAgentURL
		}
	}
	for k, v := range c.Extra {
		m[k] = v
	}
	return m
}

func platformName(p PlatformType) string {
	switch p {
	case PlatformIOS:
		return "iOS"
	case PlatformAndroid, PlatformAndroidTV:
		return "Android"
	}
	return string(p)
}
