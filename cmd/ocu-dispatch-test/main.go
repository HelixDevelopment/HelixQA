// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command ocu-dispatch-test drives the Dispatcher against the
// configured hosts and prints which host a CUDA-OpenCV capability
// was resolved to. It intentionally does NOT execute work yet — the
// actual CUDA sidecar + gRPC transport land in P2.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"digital.vasic.containers/pkg/scheduler"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	ocuremote "digital.vasic.helixqa/pkg/nexus/native/remote"
)

func main() {
	if err := run(context.Background(), os.Stdout, nil); err != nil {
		fmt.Fprintln(os.Stderr, "ocu-dispatch-test:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out io.Writer, hm ocuremote.HostManager) error {
	if hm == nil {
		return fmt.Errorf("HostManager is nil (set CONTAINERS_REMOTE_* env and wire a Containers HostManager in main)")
	}
	d := ocuremote.NewDispatcher(hm, scheduler.Options{GPUWeight: 1})
	w, err := d.Resolve(ctx, contracts.Capability{
		Kind:    contracts.KindCUDAOpenCV,
		MinVRAM: 2048,
	})
	if err != nil {
		return err
	}
	defer w.Close()
	if info, ok := ocuremote.Unwrap(w); ok {
		fmt.Fprintf(out, "dispatcher resolved to host=%s\n", info.Host())
		return nil
	}
	fmt.Fprintln(out, "dispatcher resolved to local worker")
	return nil
}
