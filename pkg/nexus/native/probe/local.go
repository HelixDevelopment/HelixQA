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

// readLocalMemoryMB returns total host RAM in MB, or 0 if unknown.
// Linux: parses /proc/meminfo `MemTotal:` line (kB → MB).
// macOS: shells out to `sysctl -n hw.memsize` (bytes → MB).
// Other:  returns 0. Callers MUST treat 0 as "unknown", not "no RAM".
//
// Fixed iter 31: original implementation was Linux-only, which made
// TestProbeLocal_PopulatesHost + TestStress_ProbeLocal_Concurrent fail
// on every macOS host. Adding the macOS branch removes the false
// "MemoryTotalMB=0 on supported platform" report.
func readLocalMemoryMB() uint64 {
	switch runtime.GOOS {
	case "linux":
		return readLinuxMemoryMB()
	case "darwin":
		return readDarwinMemoryMB()
	default:
		return 0
	}
}

func readLinuxMemoryMB() uint64 {
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

func readDarwinMemoryMB() uint64 {
	out, err := execOutput(context.Background(), "sysctl", "-n", "hw.memsize")
	if err != nil {
		return 0
	}
	bytes := parseUint(strings.TrimSpace(out))
	if bytes == 0 {
		return 0
	}
	// hw.memsize is in bytes; convert to MB (1 MB = 1024*1024).
	return bytes / (1024 * 1024)
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
