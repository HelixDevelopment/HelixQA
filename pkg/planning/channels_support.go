// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import "strings"

// HasAndroidTVChannelsSupport returns true when the caller-supplied
// platform list declares Android TV. The check is case-insensitive so
// integrators can use "androidtv", "AndroidTV", or "ANDROIDTV" and
// still have the Channels-feature test generation kick in. The helper
// holds no project-specific knowledge — it is purely a platform-name
// probe, reusable by any app that targets Android TV.
func HasAndroidTVChannelsSupport(platforms []string) bool {
	for _, p := range platforms {
		if strings.EqualFold(strings.TrimSpace(p), "androidtv") {
			return true
		}
	}
	return false
}
