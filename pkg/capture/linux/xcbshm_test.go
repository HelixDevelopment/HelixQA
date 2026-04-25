// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"errors"
	"strings"
	"testing"
)

func TestXCBShmFactory_ReturnsClearError(t *testing.T) {
	_, err := XCBShmFactory(Config{Width: 1, Height: 1})
	if !errors.Is(err, ErrXCBShmNotImplemented) {
		t.Errorf("want ErrXCBShmNotImplemented, got %v", err)
	}
	if !strings.Contains(err.Error(), "helixqa-x11grab") {
		t.Errorf("error should mention helixqa-x11grab alternative: %v", err)
	}
}

func TestXCBShmFactory_IsBackendFactory(t *testing.T) {
	var _ BackendFactory = XCBShmFactory
}
