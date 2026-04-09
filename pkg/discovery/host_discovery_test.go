package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHostDiscovery(t *testing.T) {
	hd := NewHostDiscovery()
	assert.NotNil(t, hd)
	assert.NotNil(t, hd.hosts)
	assert.Empty(t, hd.GetHosts())
}

func TestHostDiscovery_GetHosts(t *testing.T) {
	hd := NewHostDiscovery()

	// Add test hosts
	hd.mu.Lock()
	hd.hosts["192.168.1.10"] = &HostCapabilities{
		IP:       "192.168.1.10",
		Hostname: "test-host-1",
		CPUCount: 4,
		TotalRAM: 8192,
	}
	hd.hosts["192.168.1.11"] = &HostCapabilities{
		IP:       "192.168.1.11",
		Hostname: "test-host-2",
		CPUCount: 8,
		TotalRAM: 16384,
	}
	hd.mu.Unlock()

	hosts := hd.GetHosts()
	assert.Len(t, hosts, 2)
}

func TestHostDiscovery_GetHost(t *testing.T) {
	hd := NewHostDiscovery()

	// Add test host
	hd.mu.Lock()
	hd.hosts["192.168.1.10"] = &HostCapabilities{
		IP:       "192.168.1.10",
		Hostname: "test-host",
	}
	hd.mu.Unlock()

	// Existing host
	host, ok := hd.GetHost("192.168.1.10")
	assert.True(t, ok)
	assert.Equal(t, "test-host", host.Hostname)

	// Non-existing host
	_, ok = hd.GetHost("192.168.1.99")
	assert.False(t, ok)
}

func TestHostDiscovery_GetOptimalHost(t *testing.T) {
	hd := NewHostDiscovery()

	// Add test hosts with different capabilities
	hd.mu.Lock()
	hd.hosts["192.168.1.10"] = &HostCapabilities{
		IP:           "192.168.1.10",
		Hostname:     "cpu-host",
		CPUCount:     4,
		TotalRAM:     4096,
		GPUAvailable: false,
		LatencyMs:    10.0,
	}
	hd.hosts["192.168.1.11"] = &HostCapabilities{
		IP:           "192.168.1.11",
		Hostname:     "gpu-host",
		CPUCount:     8,
		TotalRAM:     16384,
		GPUAvailable: true,
		GPUVRAM:      8192,
		LatencyMs:    5.0,
	}
	hd.mu.Unlock()

	tests := []struct {
		name        string
		req         ResourceRequirements
		expectHost  string
		expectError bool
	}{
		{
			name: "CPU only workload",
			req: ResourceRequirements{
				MinCPUs: 2,
				MinRAM:  2048,
			},
			expectHost:  "gpu-host", // Lower latency
			expectError: false,
		},
		{
			name: "GPU required workload",
			req: ResourceRequirements{
				NeedsGPU: true,
				GPUVRAM:  4096,
			},
			expectHost:  "gpu-host",
			expectError: false,
		},
		{
			name: "GPU with high VRAM requirement",
			req: ResourceRequirements{
				NeedsGPU: true,
				GPUVRAM:  16000, // Higher than available
			},
			expectHost:  "",
			expectError: true,
		},
		{
			name: "High RAM requirement",
			req: ResourceRequirements{
				MinRAM: 32000,
			},
			expectHost:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := hd.GetOptimalHost(tt.req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, host)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, host)
				assert.Equal(t, tt.expectHost, host.Hostname)
			}
		})
	}
}

func TestHostDiscovery_GetHostsByCapability(t *testing.T) {
	hd := NewHostDiscovery()

	// Add test hosts
	hd.mu.Lock()
	hd.hosts["192.168.1.10"] = &HostCapabilities{
		IP:           "192.168.1.10",
		Hostname:     "basic-host",
		GPUAvailable: false,
		HasOllama:    false,
		Containers:   false,
	}
	hd.hosts["192.168.1.11"] = &HostCapabilities{
		IP:           "192.168.1.11",
		Hostname:     "gpu-host",
		GPUAvailable: true,
		HasOllama:    true,
		Containers:   true,
	}
	hd.hosts["192.168.1.12"] = &HostCapabilities{
		IP:           "192.168.1.12",
		Hostname:     "container-host",
		GPUAvailable: false,
		HasOllama:    false,
		Containers:   true,
	}
	hd.mu.Unlock()

	// Test GPU filtering
	gpuHosts := hd.GetHostsByCapability(true, false, false)
	assert.Len(t, gpuHosts, 1)
	assert.Equal(t, "gpu-host", gpuHosts[0].Hostname)

	// Test container filtering
	containerHosts := hd.GetHostsByCapability(false, false, true)
	assert.Len(t, containerHosts, 2)

	// Test combined filtering
	fullHosts := hd.GetHostsByCapability(true, true, true)
	assert.Len(t, fullHosts, 1)
	assert.Equal(t, "gpu-host", fullHosts[0].Hostname)
}

func TestHostDiscovery_meetsRequirements(t *testing.T) {
	hd := NewHostDiscovery()

	hostWithGPU := &HostCapabilities{
		CPUCount:     8,
		TotalRAM:     16384,
		GPUAvailable: true,
		GPUVRAM:      8192,
	}

	hostWithoutGPU := &HostCapabilities{
		CPUCount:     4,
		TotalRAM:     8192,
		GPUAvailable: false,
	}

	tests := []struct {
		name     string
		host     *HostCapabilities
		req      ResourceRequirements
		expected bool
	}{
		{
			name:     "no requirements",
			host:     hostWithGPU,
			req:      ResourceRequirements{},
			expected: true,
		},
		{
			name: "meets CPU requirement",
			host: hostWithGPU,
			req: ResourceRequirements{
				MinCPUs: 4,
			},
			expected: true,
		},
		{
			name: "fails CPU requirement",
			host: hostWithGPU,
			req: ResourceRequirements{
				MinCPUs: 16,
			},
			expected: false,
		},
		{
			name: "meets RAM requirement",
			host: hostWithGPU,
			req: ResourceRequirements{
				MinRAM: 8192,
			},
			expected: true,
		},
		{
			name: "fails RAM requirement",
			host: hostWithGPU,
			req: ResourceRequirements{
				MinRAM: 32000,
			},
			expected: false,
		},
		{
			name: "meets GPU requirement",
			host: hostWithGPU,
			req: ResourceRequirements{
				NeedsGPU: true,
				GPUVRAM:  4096,
			},
			expected: true,
		},
		{
			name: "fails GPU requirement - no GPU",
			host: hostWithoutGPU,
			req: ResourceRequirements{
				NeedsGPU: true,
			},
			expected: false,
		},
		{
			name: "fails GPU VRAM requirement",
			host: hostWithGPU,
			req: ResourceRequirements{
				NeedsGPU: true,
				GPUVRAM:  16000,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hd.meetsRequirements(tt.host, tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNetworkScanner_PingSweep(t *testing.T) {
	scanner := NewNetworkScanner()
	scanner.SetTimeout(500 * time.Millisecond)

	// Test with localhost only
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hosts, err := scanner.PingSweep(ctx, "127.0.0.1/32")
	require.NoError(t, err)

	// Should find at least localhost (may be flaky in some environments)
	found := false
	for _, h := range hosts {
		if h == "127.0.0.1" {
			found = true
			break
		}
	}
	
	// Log but don't fail - ping might be restricted in some environments
	if !found {
		t.Logf("PingSweep did not find localhost. Hosts found: %v. This may be expected in restricted environments.", hosts)
	}
}

func TestNetworkScanner_pingHost(t *testing.T) {
	scanner := NewNetworkScanner()
	scanner.SetTimeout(1 * time.Second)

	ctx := context.Background()

	// Test localhost (should succeed)
	alive := scanner.pingHost(ctx, "127.0.0.1")
	assert.True(t, alive)

	// Test non-routable address (should fail)
	alive = scanner.pingHost(ctx, "192.0.2.1") // TEST-NET-1
	assert.False(t, alive)
}

func TestParseSubnet(t *testing.T) {
	subnets, err := ParseSubnet()

	// May fail in some test environments
	if err != nil {
		t.Skipf("ParseSubnet failed (may be expected in CI): %v", err)
	}

	// Should find at least one subnet
	assert.NotEmpty(t, subnets)

	// Verify format
	for _, subnet := range subnets {
		assert.Contains(t, subnet, "/")
		assert.True(t, IsPrivateIP(ExtractIPFromCIDR(subnet)) || ExtractIPFromCIDR(subnet) == "0.0.0.0")
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"127.0.0.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"256.256.256.256", false}, // invalid
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := IsPrivateIP(tt.ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractIPFromCIDR(t *testing.T) {
	tests := []struct {
		cidr     string
		expected string
	}{
		{"192.168.1.0/24", "192.168.1.0"},
		{"10.0.0.0/8", "10.0.0.0"},
		{"invalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			result := ExtractIPFromCIDR(tt.cidr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLocalSubnet(t *testing.T) {
	tests := []struct {
		ip       string
		expected string
	}{
		{"192.168.1.100", "192.168.1.0/24"},
		{"10.0.0.50", "10.0.0.0/24"},
		{"172.16.5.1", "172.16.5.0/24"},
		{"invalid", ""},
		{"::1", ""}, // IPv6
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := GetLocalSubnet(tt.ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseOllamaModels(t *testing.T) {
	input := `NAME                        ID              SIZE      MODIFIED               
llava:13b-v1.6              1234567890ab    7.5 GB    2 days ago            
qwen2-vl:latest             abcdef123456    4.2 GB    5 days ago            
llama3.2:latest             fedcba098765    2.0 GB    1 week ago            `

	models := ParseOllamaModels(input)
	assert.Len(t, models, 3)
	assert.Contains(t, models, "llava:13b-v1.6")
	assert.Contains(t, models, "qwen2-vl:latest")
	assert.Contains(t, models, "llama3.2:latest")
}

func TestParseOllamaModels_Empty(t *testing.T) {
	input := "NAME                        ID              SIZE      MODIFIED"
	models := ParseOllamaModels(input)
	assert.Empty(t, models)
}

func TestHostDiscovery_RemoveHost(t *testing.T) {
	hd := NewHostDiscovery()

	// Add and then remove
	hd.mu.Lock()
	hd.hosts["192.168.1.10"] = &HostCapabilities{IP: "192.168.1.10"}
	hd.mu.Unlock()

	assert.Len(t, hd.GetHosts(), 1)

	hd.RemoveHost("192.168.1.10")
	assert.Empty(t, hd.GetHosts())

	// Removing non-existent should not panic
	hd.RemoveHost("non-existent")
}

func TestHostDiscovery_MarshalJSON(t *testing.T) {
	hd := NewHostDiscovery()

	// Add test data
	hd.mu.Lock()
	hd.hosts["192.168.1.10"] = &HostCapabilities{
		IP:       "192.168.1.10",
		Hostname: "test",
		CPUCount: 4,
	}
	hd.mu.Unlock()

	data, err := hd.MarshalJSON()
	require.NoError(t, err)

	// Verify it's valid JSON
	var result map[string]*HostCapabilities
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Len(t, result, 1)
	assert.Equal(t, "test", result["192.168.1.10"].Hostname)
}

func TestAutoDiscover(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// This may or may not find hosts depending on network
	hd, err := AutoDiscover(ctx)

	// Should not error, might just find localhost
	if err != nil {
		t.Logf("AutoDiscover returned error (may be expected): %v", err)
		return
	}

	assert.NotNil(t, hd)
	// May find 0 or more hosts depending on network
	t.Logf("Found %d hosts", len(hd.GetHosts()))
}

func TestHostDiscovery_getLocalHostInfo(t *testing.T) {
	hd := NewHostDiscovery()

	caps := &HostCapabilities{
		IP: "127.0.0.1",
	}

	err := hd.getLocalHostInfo(caps)
	require.NoError(t, err)

	// Verify basic info was populated
	assert.NotEmpty(t, caps.Hostname)
	assert.Equal(t, runtime.NumCPU(), caps.CPUCount)
	t.Logf("Local host: %s, CPUs: %d, RAM: %d MB", caps.Hostname, caps.CPUCount, caps.TotalRAM)
}

func TestHostDiscovery_getLocalRAM(t *testing.T) {
	hd := NewHostDiscovery()
	ram := hd.getLocalRAM()

	if runtime.GOOS == "linux" {
		// Should detect RAM on Linux
		assert.Greater(t, ram, uint64(0), "should detect RAM on Linux")
	} else {
		// May return 0 on other platforms
		t.Logf("RAM detection: %d MB (may be 0 on non-Linux)", ram)
	}
}

func BenchmarkPingSweep(b *testing.B) {
	scanner := NewNetworkScanner()
	scanner.SetTimeout(100 * time.Millisecond)
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		// Use /30 for fast benchmark (only 2 hosts)
		_, _ = scanner.PingSweep(ctx, "127.0.0.1/30")
	}
}

func BenchmarkGetOptimalHost(b *testing.B) {
	hd := NewHostDiscovery()

	// Populate with test data
	hd.mu.Lock()
	for i := 0; i < 100; i++ {
		hd.hosts[fmt.Sprintf("192.168.1.%d", i)] = &HostCapabilities{
			IP:           fmt.Sprintf("192.168.1.%d", i),
			Hostname:     fmt.Sprintf("host-%d", i),
			CPUCount:     4 + (i % 8),
			TotalRAM:     uint64(4096 + (i%4)*4096),
			GPUAvailable: i%10 == 0,
			LatencyMs:    float64(i),
		}
	}
	hd.mu.Unlock()

	req := ResourceRequirements{
		NeedsGPU: true,
		MinRAM:   8192,
		MinCPUs:  6,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hd.GetOptimalHost(req)
	}
}
