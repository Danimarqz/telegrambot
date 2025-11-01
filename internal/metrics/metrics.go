package metrics

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// Options controls the behaviour of the metrics gathering process.
type Options struct {
	DiskTargets    []string
	SampleInterval time.Duration
}

// Collector collects system metrics in a single coherent call.
type Collector struct {
	options Options
}

// NewCollector builds a Collector applying sensible defaults.
func NewCollector(opts Options) *Collector {
	if opts.SampleInterval <= 0 {
		opts.SampleInterval = time.Second
	}
	if len(opts.DiskTargets) == 0 {
		opts.DiskTargets = []string{"/"}
	}

	return &Collector{options: opts}
}

// SampleInterval exposes the interval used for differential metrics.
func (c *Collector) SampleInterval() time.Duration {
	return c.options.SampleInterval
}

// Stats represents a snapshot of the server state.
type Stats struct {
	CPU      CPUStats
	Memory   MemoryStats
	Disks    []DiskUsage
	Network  NetworkStats
	IO       DiskIOStats
	GPU      []GPUStats
	Host     HostStats
	Warnings []string
}

type CPUStats struct {
	Usage     float64
	Load1     float64
	Load5     float64
	Load15    float64
	Cores     int
	LoadRatio float64
}

type MemoryStats struct {
	Used        uint64
	Total       uint64
	UsedPercent float64
	SwapUsed    uint64
	SwapTotal   uint64
	SwapPercent float64
}

type DiskUsage struct {
	Mount       string
	Used        uint64
	Total       uint64
	UsedPercent float64
}

type NetworkStats struct {
	SentPerSec     uint64
	ReceivedPerSec uint64
}

type DiskIOStats struct {
	ReadPerSec  uint64
	WritePerSec uint64
}

type GPUStats struct {
	Index       string
	Name        string
	Utilization string
	MemoryUsed  string
	MemoryTotal string
	Temperature string
}

type HostStats struct {
	Uptime time.Duration
}

// Collect gathers metrics, recording warnings for partial failures instead of aborting.
func (c *Collector) Collect(ctx context.Context) (Stats, error) {
	var stats Stats

	cpuStats, err := collectCPU(ctx)
	if err != nil {
		stats.Warnings = append(stats.Warnings, fmt.Sprintf("CPU: %v", err))
	} else {
		stats.CPU = cpuStats
	}

	memStats, err := collectMemory(ctx)
	if err != nil {
		stats.Warnings = append(stats.Warnings, fmt.Sprintf("Memoria: %v", err))
	} else {
		stats.Memory = memStats
	}

	diskStats, err := c.collectDisks(ctx)
	if err != nil {
		stats.Warnings = append(stats.Warnings, fmt.Sprintf("Discos: %v", err))
	} else {
		stats.Disks = diskStats
	}

	networkStats, err := c.collectNetwork(ctx)
	if err != nil {
		stats.Warnings = append(stats.Warnings, fmt.Sprintf("Red: %v", err))
	} else {
		stats.Network = networkStats
	}

	diskIOStats, err := c.collectDiskIO(ctx)
	if err != nil {
		stats.Warnings = append(stats.Warnings, fmt.Sprintf("Disco IO: %v", err))
	} else {
		stats.IO = diskIOStats
	}

	gpuStats, err := collectGPU(ctx)
	if err != nil {
		stats.Warnings = append(stats.Warnings, fmt.Sprintf("GPU: %v", err))
	} else {
		stats.GPU = gpuStats
	}

	hostStats, err := collectHost(ctx)
	if err != nil {
		stats.Warnings = append(stats.Warnings, fmt.Sprintf("Host: %v", err))
	} else {
		stats.Host = hostStats
	}

	return stats, nil
}

func collectCPU(ctx context.Context) (CPUStats, error) {
	if err := ctx.Err(); err != nil {
		return CPUStats{}, err
	}

	perc, err := cpu.Percent(0, false)
	if err != nil {
		return CPUStats{}, err
	}
	if len(perc) == 0 {
		return CPUStats{}, fmt.Errorf("no CPU data")
	}

	loadAvg, err := load.Avg()
	if err != nil {
		return CPUStats{}, err
	}

	cores, err := cpu.Counts(true)
	if err != nil {
		return CPUStats{}, err
	}

	loadRatio := 0.0
	if cores > 0 {
		loadRatio = (loadAvg.Load1 / float64(cores)) * 100
	}

	return CPUStats{
		Usage:     perc[0],
		Load1:     loadAvg.Load1,
		Load5:     loadAvg.Load5,
		Load15:    loadAvg.Load15,
		Cores:     cores,
		LoadRatio: loadRatio,
	}, nil
}

func collectMemory(ctx context.Context) (MemoryStats, error) {
	if err := ctx.Err(); err != nil {
		return MemoryStats{}, err
	}

	vmem, err := mem.VirtualMemory()
	if err != nil {
		return MemoryStats{}, err
	}

	swap, err := mem.SwapMemory()
	if err != nil {
		return MemoryStats{}, err
	}

	return MemoryStats{
		Used:        vmem.Used,
		Total:       vmem.Total,
		UsedPercent: vmem.UsedPercent,
		SwapUsed:    swap.Used,
		SwapTotal:   swap.Total,
		SwapPercent: swap.UsedPercent,
	}, nil
}

func (c *Collector) collectDisks(ctx context.Context) ([]DiskUsage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	targetSet := make(map[string]struct{}, len(c.options.DiskTargets))
	for _, t := range c.options.DiskTargets {
		targetSet[strings.TrimSpace(t)] = struct{}{}
	}

	var usages []DiskUsage
	for _, partition := range partitions {
		if _, ok := targetSet[partition.Mountpoint]; !ok {
			continue
		}

		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			return nil, fmt.Errorf("mount %s: %w", partition.Mountpoint, err)
		}

		usages = append(usages, DiskUsage{
			Mount:       partition.Mountpoint,
			Used:        usage.Used,
			Total:       usage.Total,
			UsedPercent: usage.UsedPercent,
		})
	}

	sort.Slice(usages, func(i, j int) bool {
		return usages[i].Mount < usages[j].Mount
	})

	return usages, nil
}

func (c *Collector) collectNetwork(ctx context.Context) (NetworkStats, error) {
	if err := ctx.Err(); err != nil {
		return NetworkStats{}, err
	}

	first, err := net.IOCounters(false)
	if err != nil {
		return NetworkStats{}, err
	}
	if len(first) == 0 {
		return NetworkStats{}, fmt.Errorf("no network data")
	}

	timer := time.NewTimer(c.options.SampleInterval)
	select {
	case <-ctx.Done():
		timer.Stop()
		return NetworkStats{}, ctx.Err()
	case <-timer.C:
	}

	second, err := net.IOCounters(false)
	if err != nil {
		return NetworkStats{}, err
	}
	if len(second) == 0 {
		return NetworkStats{}, fmt.Errorf("no network data")
	}

	return NetworkStats{
		SentPerSec:     second[0].BytesSent - first[0].BytesSent,
		ReceivedPerSec: second[0].BytesRecv - first[0].BytesRecv,
	}, nil
}

func (c *Collector) collectDiskIO(ctx context.Context) (DiskIOStats, error) {
	if err := ctx.Err(); err != nil {
		return DiskIOStats{}, err
	}

	first, err := disk.IOCounters()
	if err != nil {
		return DiskIOStats{}, err
	}

	timer := time.NewTimer(c.options.SampleInterval)
	select {
	case <-ctx.Done():
		timer.Stop()
		return DiskIOStats{}, ctx.Err()
	case <-timer.C:
	}

	second, err := disk.IOCounters()
	if err != nil {
		return DiskIOStats{}, err
	}

	for name, current := range second {
		previous, ok := first[name]
		if !ok {
			continue
		}
		return DiskIOStats{
			ReadPerSec:  current.ReadBytes - previous.ReadBytes,
			WritePerSec: current.WriteBytes - previous.WriteBytes,
		}, nil
	}

	return DiskIOStats{}, fmt.Errorf("no disk io data")
}

func collectGPU(ctx context.Context) ([]GPUStats, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total,temperature.gpu",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(bytes.TrimSpace(output), []byte{'\n'})
	if len(lines) == 0 || len(lines[0]) == 0 {
		return nil, nil
	}

	stats := make([]GPUStats, 0, len(lines))
	for _, line := range lines {
		fields := bytes.Split(bytes.TrimSpace(line), []byte{','})
		if len(fields) < 6 {
			continue
		}

		stats = append(stats, GPUStats{
			Index:       string(bytes.TrimSpace(fields[0])),
			Name:        string(bytes.TrimSpace(fields[1])),
			Utilization: string(bytes.TrimSpace(fields[2])) + "%",
			MemoryUsed:  string(bytes.TrimSpace(fields[3])) + "MiB",
			MemoryTotal: string(bytes.TrimSpace(fields[4])) + "MiB",
			Temperature: string(bytes.TrimSpace(fields[5])),
		})
	}

	return stats, nil
}

func collectHost(ctx context.Context) (HostStats, error) {
	if err := ctx.Err(); err != nil {
		return HostStats{}, err
	}

	info, err := host.Info()
	if err != nil {
		return HostStats{}, err
	}
	return HostStats{
		Uptime: time.Duration(info.Uptime) * time.Second,
	}, nil
}

// FormatHTML renders the stats into Telegram-ready HTML.
func FormatHTML(stats Stats) string {
	var buf strings.Builder
	buf.WriteString("<b>Server Stats</b>\n\n")

	if stats.CPU.Cores > 0 {
		buf.WriteString(fmt.Sprintf("CPU: %.1f%% | Load %.2f / %.2f / %.2f (%.0f%% uso global)\n",
			stats.CPU.Usage, stats.CPU.Load1, stats.CPU.Load5, stats.CPU.Load15, stats.CPU.LoadRatio))
	}

	if stats.Memory.Total > 0 {
		buf.WriteString(fmt.Sprintf("Mem: %s/%s (%.1f%%) | Swap: %s/%s (%.1f%%)\n",
			human(stats.Memory.Used), human(stats.Memory.Total), stats.Memory.UsedPercent,
			human(stats.Memory.SwapUsed), human(stats.Memory.SwapTotal), stats.Memory.SwapPercent))
	}

	if stats.Network.SentPerSec > 0 || stats.Network.ReceivedPerSec > 0 || (stats.IO.ReadPerSec > 0 || stats.IO.WritePerSec > 0) {
		buf.WriteString(fmt.Sprintf("Net Up %s/s Down %s/s | IO R:%s/s W:%s/s\n",
			human(stats.Network.SentPerSec), human(stats.Network.ReceivedPerSec),
			human(stats.IO.ReadPerSec), human(stats.IO.WritePerSec)))
	}

	if len(stats.Disks) > 0 {
		for _, disk := range stats.Disks {
			buf.WriteString(fmt.Sprintf("%s %s/%s (%.1f%%)\n",
				disk.Mount, human(disk.Used), human(disk.Total), disk.UsedPercent))
		}
	}

	if len(stats.GPU) > 0 {
		for _, gpu := range stats.GPU {
			buf.WriteString(fmt.Sprintf("GPU%s %s | Util %s | Mem %s/%s | Temp %sC\n",
				gpu.Index, gpu.Name, gpu.Utilization, gpu.MemoryUsed, gpu.MemoryTotal, gpu.Temperature))
		}
	} else {
		buf.WriteString("GPU: no disponible\n")
	}

	if stats.Host.Uptime > 0 {
		buf.WriteString(fmt.Sprintf("Uptime: %s\n", stats.Host.Uptime.Truncate(time.Second)))
	}

	if len(stats.Warnings) > 0 {
		buf.WriteString("\n<b>Advertencias</b>\n")
		for _, warning := range stats.Warnings {
			buf.WriteString("- ")
			buf.WriteString(html.EscapeString(warning))
			buf.WriteByte('\n')
		}
	}

	return buf.String()
}

func human(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
		if exp >= len("KMGTPE") {
			break
		}
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
