// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// CapabilityKind identifies a hardware capability that a remote worker must
// provide.
type CapabilityKind string

const (
	// KindCUDAOpenCV requires an NVIDIA GPU with CUDA-accelerated OpenCV.
	KindCUDAOpenCV CapabilityKind = "cuda-opencv"

	// KindNVENC requires NVIDIA hardware video encoding (NVENC).
	KindNVENC CapabilityKind = "nvenc"

	// KindTensorRTOCR requires TensorRT-optimised OCR inference.
	KindTensorRTOCR CapabilityKind = "tensorrt-ocr"
)

// Capability describes the minimum hardware requirement that a Dispatcher must
// satisfy when resolving a Worker.
type Capability struct {
	Kind        CapabilityKind
	MinVRAM     int  // minimum VRAM in MiB; 0 means no requirement
	PreferLocal bool // prefer a worker on the same host when true
}

// Worker is a handle to a remote-dispatch target that communicates via
// Protocol Buffers over gRPC or an equivalent transport.
type Worker interface {
	// Call sends req to the worker, waits for a response, and unmarshals it
	// into resp.  Both req and resp must be valid proto.Message values.
	Call(ctx context.Context, req proto.Message, resp proto.Message) error

	// Close releases the connection back to the pool or closes it outright.
	Close() error
}

// Dispatcher resolves the best available Worker for a given Capability set.
// Implementations may consult a service registry, a local capability probe,
// or a static configuration map.
type Dispatcher interface {
	// Resolve returns a Worker that satisfies need, or an error if none is
	// available.  The caller must call Worker.Close when done.
	Resolve(ctx context.Context, need Capability) (Worker, error)
}
