// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command ocu-probe prints the local host's OCU capabilities plus
// any configured remote hosts (driven by CONTAINERS_REMOTE_* env
// vars) as a single JSON document.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"digital.vasic.containers/pkg/envconfig"
	cremote "digital.vasic.containers/pkg/remote"

	"digital.vasic.helixqa/pkg/nexus/native/probe"
)

type probeOutput struct {
	Local  *probe.Report   `json:"local"`
	Remote []*probe.Report `json:"remote,omitempty"`
}

func main() {
	if err := run(context.Background(), os.Stdout, nil); err != nil {
		fmt.Fprintln(os.Stderr, "ocu-probe:", err)
		os.Exit(1)
	}
}

// run is separated from main() for testability. `exec` may be nil;
// when nil, remote hosts from env-config are skipped (local-only).
func run(ctx context.Context, out io.Writer, exec cremote.RemoteExecutor) error {
	local, err := probe.ProbeLocal(ctx)
	if err != nil {
		return fmt.Errorf("probe local: %w", err)
	}
	result := probeOutput{Local: local}

	if exec != nil {
		cfg, perr := envconfig.Parse()
		if perr == nil && cfg.Enabled {
			for _, h := range cfg.ToRemoteHosts() {
				rep, rerr := probe.ProbeRemote(ctx, exec, h)
				if rerr != nil {
					rep = &probe.Report{Host: h.Name}
				}
				result.Remote = append(result.Remote, rep)
			}
		}
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
