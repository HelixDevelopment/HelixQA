// Package mobile is the Nexus mobile engine. It wraps an Appium 2.0
// WebDriver HTTP client and builds capability profiles for iOS (XCUITest),
// Android (UiAutomator2), and Android TV (UiAutomator2 + leanback).
//
// The package is pure Go and CGo-free; it talks to an Appium server over
// HTTP/JSON so it works against simulators, emulators, and real devices
// without depending on any native SDK. Build tags are reserved for future
// work that needs a real client library:
//
//	nexus_appium_real  activates wrappers that shell out to appium CLIs
//
// Without the tag, the default client is the HTTP driver and every test
// stands up an httptest.Server so no real Appium hub is required.
package mobile
