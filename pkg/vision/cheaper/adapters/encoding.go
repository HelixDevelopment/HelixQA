// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package adapters contains shared utilities for vision provider adapters.
package adapters

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
)

// ImageToBase64 encodes an image.Image to a PNG byte stream and returns
// the result as a standard base64-encoded string. The caller can embed
// the returned string directly in a data URI or JSON payload.
func ImageToBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
