// Package discovery provides network host discovery and capability detection
// for distributed video processing workloads.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HostCapabilities describes what a host can do
type HostCapabilities struct {
	IP           string    `json:"ip"`
	Hostname     string    `json:"hostname"`
	CPUCount     int       `json:"cpu_count"`
	TotalRAM     uint64    `json:"total_ram_mb"`
	GPUAvailable bool      `json:"gpu_available"`
	GPUModel     string    `json:"gpu_model,omitempty"`
	GPUVRAM      uint64    `json:"gpu_vram_mb,omitempty"`
	LatencyMs    float64   `json:"latency_ms"`
	Containers   bool      `json:"containers_supported"`
	HasOllama    bool      `json:"has_ollama"`
	LastSeen     time.Time `json:"last_seen"`
}

// ResourceRequirements for workload placement
type ResourceRequirements struct {
	NeedsGPU bool
	MinRAM   uint64 // MB
	MinCPUs  int
	GPUVRAM  uint64 // MB, if NeedsGPU
}

// NetworkScanner performs network discovery
type NetworkScanner struct {
	timeout time.Duration
}

// NewNetworkScanner creates a new scanner with default timeout
func NewNetworkScanner() *NetworkScanner {
	return &NetworkScanner{
		timeout: 2 * time.Second,
	}
}

// SetTimeout configures the scan timeout
func (ns *NetworkScanner) SetTimeout(d time.Duration) {
	ns.timeout = d
}

// PingSweep scans a subnet and returns live hosts
func (ns *NetworkScanner) PingSweep(ctx context.Context, subnet string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet %s: %w", subnet, err)
	}

	var hosts []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent pings
	semaphore := make(chan struct{}, 50)

	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		if ip.Equal(ipnet.IP) || ip.Equal(broadcastIP(ipnet)) {
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(target string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			if alive := ns.pingHost(ctx, target); alive {
				mu.Lock()
				hosts = append(hosts, target)
				mu.Unlock()
			}
		}(ip.String())
	}

	wg.Wait()
	return hosts, nil
}

// pingHost checks if a host responds to ping
func (ns *NetworkScanner) pingHost(ctx context.Context, host string) bool {
	ctx, cancel := context.WithTimeout(ctx, ns.timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", "1000", host)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", host)
	}

	err := cmd.Run()
	return err == nil
}

// incIP increments an IP address
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// broadcastIP calculates broadcast address
func broadcastIP(n *net.IPNet) net.IP {
	broadcast := make(net.IP, len(n.IP))
	copy(broadcast, n.IP)
	for i := range broadcast {
		broadcast[i] |= ^n.Mask[i]
	}
	return broadcast
}

// HostDiscovery manages discovered hosts
type HostDiscovery struct {
	mu      sync.RWMutex
	hosts   map[string]*HostCapabilities
	scanner *NetworkScanner
}

// NewHostDiscovery creates discovery service
func NewHostDiscovery() *HostDiscovery {
	return &HostDiscovery{
		hosts:   make(map[string]*HostCapabilities),
		scanner: NewNetworkScanner(),
	}
}

// ScanNetwork finds hosts in subnet and probes their capabilities
func (hd *HostDiscovery) ScanNetwork(ctx context.Context, subnet string) ([]*HostCapabilities, error) {
	// Find live hosts
	liveHosts, err := hd.scanner.PingSweep(ctx, subnet)
	if err != nil {
		return nil, err
	}

	// Probe each host for capabilities
	var discovered []*HostCapabilities
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, host := range liveHosts {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()

			caps := hd.probeHost(ctx, ip)
			if caps != nil {
				mu.Lock()
				discovered = append(discovered, caps)
				mu.Unlock()

				hd.mu.Lock()
				hd.hosts[ip] = caps
				hd.mu.Unlock()
			}
		}(host)
	}

	wg.Wait()
	return discovered, nil
}

// probeHost attempts to determine host capabilities
func (hd *HostDiscovery) probeHost(ctx context.Context, ip string) *HostCapabilities {
	caps := &HostCapabilities{
		IP:       ip,
		LastSeen: time.Now(),
	}

	// Test SSH connectivity and get system info
	if err := hd.getHostInfo(ctx, caps); err != nil {
		// Host might be up but not SSH-accessible
		// Still return basic info from ping
		caps.Hostname = ip
	}

	// Measure latency
	latency, _ := hd.measureLatency(ip)
	caps.LatencyMs = latency

	return caps
}

// getHostInfo retrieves system information from host
func (hd *HostDiscovery) getHostInfo(ctx context.Context, caps *HostCapabilities) error {
	// For localhost, use direct system calls
	if caps.IP == "127.0.0.1" || caps.IP == "::1" {
		return hd.getLocalHostInfo(caps)
	}

	// For remote hosts, try SSH
	// This is a simplified version - production would use proper SSH client
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try to get hostname
	cmd := exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=2", "-o", "BatchMode=yes",
		fmt.Sprintf("root@%s", caps.IP), "hostname")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	caps.Hostname = strings.TrimSpace(string(out))

	// Get CPU count
	cmd = exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=2", "-o", "BatchMode=yes",
		fmt.Sprintf("root@%s", caps.IP), "nproc")
	out, _ = cmd.Output()
	if cpus, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
		caps.CPUCount = cpus
	}

	// Get RAM
	cmd = exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=2", "-o", "BatchMode=yes",
		fmt.Sprintf("root@%s", caps.IP), "free -m | awk '/^Mem:/{print $2}'")
	out, _ = cmd.Output()
	if ram, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
		caps.TotalRAM = ram
	}

	// Check GPU
	hd.detectGPU(ctx, caps)

	// Check container support
	hd.detectContainers(ctx, caps)

	// Check Ollama
	hd.detectOllama(ctx, caps)

	return nil
}

// getLocalHostInfo gets info for local machine
func (hd *HostDiscovery) getLocalHostInfo(caps *HostCapabilities) error {
	hostname, _ := os.Hostname()
	caps.Hostname = hostname
	caps.CPUCount = runtime.NumCPU()

	// Get RAM
	caps.TotalRAM = hd.getLocalRAM()

	// Check GPU
	hd.detectGPULocal(caps)

	// Check containers
	hd.detectContainersLocal(caps)

	// Check Ollama
	hd.detectOllamaLocal(caps)

	return nil
}

// getLocalRAM returns total system RAM in MB
func (hd *HostDiscovery) getLocalRAM() uint64 {
	// Try /proc/meminfo on Linux
	if runtime.GOOS == "linux" {
		cmd := exec.Command("awk", "/^MemTotal/{print $2}", "/proc/meminfo")
		out, err := cmd.Output()
		if err == nil {
			if kb, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return kb / 1024 // Convert KB to MB
			}
		}
	}

	// Fallback
	return 0
}

// detectGPU checks for GPU on remote host
func (hd *HostDiscovery) detectGPU(ctx context.Context, caps *HostCapabilities) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Check for NVIDIA
	cmd := exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=2", "-o", "BatchMode=yes",
		fmt.Sprintf("root@%s", caps.IP), "nvidia-smi --query-gpu=name,memory.total --format=csv,noheader 2>/dev/null")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		caps.GPUAvailable = true
		parts := strings.Split(strings.TrimSpace(string(out)), ", ")
		if len(parts) >= 2 {
			caps.GPUModel = parts[0]
			// Parse "12288 MiB" -> 12288
			vramStr := strings.TrimSpace(parts[1])
			vramStr = strings.Replace(vramStr, " MiB", "", 1)
			if vram, err := strconv.ParseUint(vramStr, 10, 64); err == nil {
				caps.GPUVRAM = vram
			}
		}
		return
	}

	// Check for AMD ROCm
	cmd = exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=2", "-o", "BatchMode=yes",
		fmt.Sprintf("root@%s", caps.IP), "rocm-smi --showproductname 2>/dev/null")
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		caps.GPUAvailable = true
		caps.GPUModel = strings.TrimSpace(string(out))
	}
}

// detectGPULocal checks for local GPU
func (hd *HostDiscovery) detectGPULocal(caps *HostCapabilities) {
	// Check NVIDIA
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		caps.GPUAvailable = true
		parts := strings.Split(strings.TrimSpace(string(out)), ", ")
		if len(parts) >= 2 {
			caps.GPUModel = parts[0]
			vramStr := strings.TrimSpace(parts[1])
			vramStr = strings.Replace(vramStr, " MiB", "", 1)
			if vram, err := strconv.ParseUint(vramStr, 10, 64); err == nil {
				caps.GPUVRAM = vram
			}
		}
		return
	}

	// Check AMD
	cmd = exec.Command("rocm-smi", "--showproductname")
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		caps.GPUAvailable = true
		caps.GPUModel = strings.TrimSpace(string(out))
	}
}

// detectContainers checks for container runtime
func (hd *HostDiscovery) detectContainers(ctx context.Context, caps *HostCapabilities) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=2", "-o", "BatchMode=yes",
		fmt.Sprintf("root@%s", caps.IP), "which podman docker 2>/dev/null")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		caps.Containers = true
	}
}

// detectContainersLocal checks for local containers
func (hd *HostDiscovery) detectContainersLocal(caps *HostCapabilities) {
	cmd := exec.Command("which", "podman")
	if err := cmd.Run(); err == nil {
		caps.Containers = true
		return
	}

	cmd = exec.Command("which", "docker")
	if err := cmd.Run(); err == nil {
		caps.Containers = true
	}
}

// detectOllama checks if Ollama is running
func (hd *HostDiscovery) detectOllama(ctx context.Context, caps *HostCapabilities) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=2", "-o", "BatchMode=yes",
		fmt.Sprintf("root@%s", caps.IP), "curl -s http://localhost:11434/api/tags >/dev/null 2>&1 && echo ok")
	out, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(out)) == "ok" {
		caps.HasOllama = true
	}
}

// detectOllamaLocal checks for local Ollama
func (hd *HostDiscovery) detectOllamaLocal(caps *HostCapabilities) {
	cmd := exec.Command("curl", "-s", "http://localhost:11434/api/tags")
	if err := cmd.Run(); err == nil {
		caps.HasOllama = true
	}
}

// measureLatency tests network latency to host
func (hd *HostDiscovery) measureLatency(host string) (float64, error) {
	// Simple TCP connect timing
	start := time.Now()
	conn, err := net.DialTimeout("tcp", host+":22", 2*time.Second)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return float64(time.Since(start).Milliseconds()), nil
}

// GetHosts returns all discovered hosts
func (hd *HostDiscovery) GetHosts() []*HostCapabilities {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	result := make([]*HostCapabilities, 0, len(hd.hosts))
	for _, h := range hd.hosts {
		result = append(result, h)
	}
	return result
}

// GetHost returns a specific host by IP
func (hd *HostDiscovery) GetHost(ip string) (*HostCapabilities, bool) {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	host, ok := hd.hosts[ip]
	return host, ok
}

// GetOptimalHost selects best host for workload
func (hd *HostDiscovery) GetOptimalHost(requirements ResourceRequirements) (*HostCapabilities, error) {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	var best *HostCapabilities
	for _, host := range hd.hosts {
		if hd.meetsRequirements(host, requirements) {
			if best == nil || host.LatencyMs < best.LatencyMs {
				best = host
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no host meets requirements: GPU=%v, RAM=%dMB, CPUs=%d",
			requirements.NeedsGPU, requirements.MinRAM, requirements.MinCPUs)
	}
	return best, nil
}

// GetHostsByCapability returns hosts that match specific criteria
func (hd *HostDiscovery) GetHostsByCapability(hasGPU, hasOllama, hasContainers bool) []*HostCapabilities {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	var result []*HostCapabilities
	for _, host := range hd.hosts {
		if hasGPU && !host.GPUAvailable {
			continue
		}
		if hasOllama && !host.HasOllama {
			continue
		}
		if hasContainers && !host.Containers {
			continue
		}
		result = append(result, host)
	}
	return result
}

func (hd *HostDiscovery) meetsRequirements(h *HostCapabilities, req ResourceRequirements) bool {
	if req.NeedsGPU && !h.GPUAvailable {
		return false
	}
	if req.NeedsGPU && req.GPUVRAM > 0 && h.GPUVRAM < req.GPUVRAM {
		return false
	}
	if req.MinRAM > 0 && h.TotalRAM < req.MinRAM {
		return false
	}
	if req.MinCPUs > 0 && h.CPUCount < req.MinCPUs {
		return false
	}
	return true
}

// RemoveHost removes a host from discovery
func (hd *HostDiscovery) RemoveHost(ip string) {
	hd.mu.Lock()
	defer hd.mu.Unlock()

	delete(hd.hosts, ip)
}

// MarshalJSON serializes discovered hosts
func (hd *HostDiscovery) MarshalJSON() ([]byte, error) {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	return json.Marshal(hd.hosts)
}

// ParseSubnet extracts local subnets from network interfaces
func ParseSubnet() ([]string, error) {
	var subnets []string

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			// Only IPv4
			if ipnet.IP.To4() == nil {
				continue
			}

			// Skip localhost
			if ipnet.IP.IsLoopback() {
				continue
			}

			// Calculate /24 subnet
			mask := net.CIDRMask(24, 32)
			network := ipnet.IP.Mask(mask)
			subnet := fmt.Sprintf("%s/24", network.String())

			// Deduplicate
			found := false
			for _, s := range subnets {
				if s == subnet {
					found = true
					break
				}
			}
			if !found {
				subnets = append(subnets, subnet)
			}
		}
	}

	return subnets, nil
}

// AutoDiscover scans local subnets automatically
func AutoDiscover(ctx context.Context) (*HostDiscovery, error) {
	subnets, err := ParseSubnet()
	if err != nil {
		return nil, fmt.Errorf("failed to parse subnets: %w", err)
	}

	if len(subnets) == 0 {
		return nil, fmt.Errorf("no valid subnets found")
	}

	hd := NewHostDiscovery()

	for _, subnet := range subnets {
		_, err := hd.ScanNetwork(ctx, subnet)
		if err != nil {
			// Log but continue
			continue
		}
	}

	return hd, nil
}

// IsPrivateIP checks if IP is in private range
func IsPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Private IP ranges
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
	}

	for _, block := range privateBlocks {
		_, ipnet, _ := net.ParseCIDR(block)
		if ipnet.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// ExtractIPFromCIDR extracts the IP part from CIDR notation
func ExtractIPFromCIDR(cidr string) string {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}
	return ip.String()
}

// ParseOllamaModels parses output from `ollama list`
func ParseOllamaModels(output string) []string {
	var models []string
	lines := strings.Split(output, "\n")

	// Skip header
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// Extract model name (first column)
		fields := strings.Fields(line)
		if len(fields) > 0 {
			models = append(models, fields[0])
		}
	}

	return models
}

// GetLocalSubnet returns the local subnet for an IP
func GetLocalSubnet(ip string) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ""
	}

	// Default to /24
	ip4 := parsedIP.To4()
	if ip4 == nil {
		return ""
	}

	return fmt.Sprintf("%d.%d.%d.0/24", ip4[0], ip4[1], ip4[2])
}

// regex for parsing nvidia-smi output
var nvidiaSmiRegex = regexp.MustCompile(`(\d+)MiB`)

// parseNvidiaMemory extracts memory from nvidia-smi output
func parseNvidiaMemory(output string) uint64 {
	matches := nvidiaSmiRegex.FindStringSubmatch(output)
	if len(matches) > 1 {
		if vram, err := strconv.ParseUint(matches[1], 10, 64); err == nil {
			return vram
		}
	}
	return 0
}
