// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"errors"
	"fmt"
)

// ErrXCBShmNotImplemented is returned by the XCBShm BackendFactory variant.
// xcbshm was listed in OpenClawing4 §5.1.1 as an optional X11 fallback, but
// modern Linux deployments almost always have either:
//
//   - Wayland + xdg-desktop-portal ScreenCast (prefer: BackendPortal)
//   - X11 or XWayland + ffmpeg x11grab (prefer: BackendX11Grab via
//     cmd/helixqa-x11grab)
//
// A pure-Go xcb-shm implementation would introduce a non-trivial X11 client
// (either a new dep like github.com/jezek/xgb or a hand-rolled XCB wire
// protocol), for a niche use case that X11Grab already covers. Rather than
// ship half-done code, this file exposes the factory signature returning
// ErrXCBShmNotImplemented so callers see a clear error if they try to opt
// in via a hypothetical BackendXCBShm value.
//
// The xcbshm path is NOT in Backend — BackendX11Grab covers all X11 capture
// surfaces HelixQA supports today. Adding xcbshm is gated on a concrete
// operator request for a particular performance / latency characteristic
// x11grab cannot meet.
var ErrXCBShmNotImplemented = errors.New("linux/capture: xcbshm backend is not implemented — use BackendX11Grab (helixqa-x11grab) or BackendPortal instead")

// XCBShmFactory is a sentinel BackendFactory that always returns
// ErrXCBShmNotImplemented. Exported so callers that carry an
// `XCBShmFactory BackendFactory` field in their own Config structs still
// compile even when they never actually use it.
//
// Returning this factory from NewSource is impossible today because the
// BackendXCBShm enum value does not exist — adding it is a future commit
// that lands alongside a real implementation.
func XCBShmFactory(_ Config) (Source, error) {
	return nil, fmt.Errorf("%w: install either pipewire+xdg-desktop-portal or ffmpeg+helixqa-x11grab", ErrXCBShmNotImplemented)
}
