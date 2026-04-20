// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package text is the HelixQA client for the text-detection sidecar.
//
// Contents:
//
//   - text.go — ✅ HTTP client for the EAST/MSER/PP-OCR Python
//               sidecar (cmd/helixqa-text/, future). Multipart PNG
//               upload, JSON region grid response, smallest-
//               enclosing FindContaining grounding helper. Same
//               wire pattern as pkg/agent/omniparser. Shipped M51.
//
// Replaces the doc-planned gocv EAST/MSER wrappers: text detection
// models require a Python CV stack (ONNX Runtime, TF) that doesn't
// fit the CGO-free Go host. The sidecar pattern keeps the Go client
// small while the Python process carries the model dependencies.
package text
