// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package probe discovers the hardware capabilities of the local
// machine and any reachable remote hosts. It is used by P0 to
// route CUDA-bound calls to the right executor, and by
// cmd/ocu-probe to produce a human-readable report.
package probe

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"

	"digital.vasic.containers/pkg/remote"
)

// Report is a single host's hardware snapshot.
type Report struct {
	Host          string
	OS            string
	Arch          string
	CPUCores      int
	MemoryTotalMB uint64
	GPU           []remote.GPUDevice
	OpenCL        bool
	Vulkan        bool
}

// ProbeLocal runs the probes on the current process's host.
func ProbeLocal(ctx context.Context) (*Report, error) {
	r := &Report{
		Host:     "local",
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCores: runtime.NumCPU(),
	}
	if mem := readLocalMemoryMB(); mem > 0 {
		r.MemoryTotalMB = mem
	}
	if devs := runLocalNvidiaSmi(ctx); len(devs) > 0 {
		r.GPU = append(r.GPU, devs...)
	}
	r.OpenCL = hasBinary("clinfo")
	r.Vulkan = hasBinary("vulkaninfo")
	return r, nil
}

func readLocalMemoryMB() uint64 {
	data, err := readFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0
			}
			// MemTotal is reported in kB.
			return parseUint(fields[1]) / 1024
		}
	}
	return 0
}

func runLocalNvidiaSmi(ctx context.Context) []remote.GPUDevice {
	out, err := execOutput(ctx,
		"nvidia-smi",
		"--query-gpu=index,name,driver_version,memory.total,memory.free,utilization.gpu,compute_cap",
		"--format=csv,noheader,nounits",
	)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	devs, err := remote.ParseNvidiaSmi(out)
	if err != nil {
		return nil
	}
	return devs
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// --- tiny helpers isolated for easy mock in tests ---

var (
	execOutput = func(ctx context.Context, name string, args ...string) (string, error) {
		var buf bytes.Buffer
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdout = &buf
		if err := cmd.Run(); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	readFile = osReadFile
)
