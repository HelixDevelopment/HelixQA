// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"context"

	"digital.vasic.containers/pkg/remote"
)

// ProbeRemote calls Containers.ProbeGPU over the supplied executor
// and returns a Report. OS/CPU/RAM fields stay zero in this minimal
// P0 impl — later phases can extend with `uname`, `/proc/meminfo`
// probes over SSH if/when needed.
func ProbeRemote(
	ctx context.Context,
	exec remote.RemoteExecutor,
	host remote.RemoteHost,
) (*Report, error) {
	devs, err := remote.ProbeGPU(ctx, exec, host)
	if err != nil {
		return nil, err
	}
	return &Report{
		Host: host.Name,
		GPU:  devs,
	}, nil
}
