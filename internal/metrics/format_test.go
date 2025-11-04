package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestFormatHTMLWithGPUAndWarnings(t *testing.T) {
	stats := Stats{
		CPU: CPUStats{
			Usage:     72.5,
			Load1:     1.23,
			Load5:     0.98,
			Load15:    0.75,
			Cores:     8,
			LoadRatio: 65,
		},
		Memory: MemoryStats{
			Used:        8 * 1024 * 1024 * 1024,
			Total:       16 * 1024 * 1024 * 1024,
			UsedPercent: 50,
			SwapUsed:    512 * 1024 * 1024,
			SwapTotal:   1024 * 1024 * 1024,
			SwapPercent: 50,
		},
		Disks: []DiskUsage{
			{Mount: "/", Used: 90 * 1024 * 1024 * 1024, Total: 128 * 1024 * 1024 * 1024, UsedPercent: 70.3},
		},
		Network: NetworkStats{
			SentPerSec:     12 * 1024,
			ReceivedPerSec: 24 * 1024,
		},
		IO: DiskIOStats{
			ReadPerSec:  1024,
			WritePerSec: 2048,
		},
		GPU: []GPUStats{
			{
				Index:       "0",
				Name:        "RTX 4090",
				Utilization: "85",
				MemoryUsed:  "1000MiB",
				MemoryTotal: "12000MiB",
				Temperature: "65",
				Power:       "250W",
			},
		},
		Host: HostStats{
			Uptime: 3*time.Hour + 4*time.Minute + 5*time.Second,
		},
		Warnings: []string{"GPU: unavailable", "Disk: slow"},
	}

	output := FormatHTML(stats)

	mustContain := []string{
		"<b>📊 Estado del Servidor</b>",
		"⚙️ <b>CPU</b>",
		"• Uso: 72.5%",
		"• Carga: 1.23 / 0.98 / 0.75",
		"• Uso global: 65%",
		"🧠 <b>Memoria</b>",
		"RAM: 8.0GB/16.0GB =&gt; (50.0%)",
		"Swap: 512.0MB/1.0GB =&gt; (50.0%)",
		"🌐 <b>Red &amp; IO</b>",
		"Red: ↑ 12.0KB/s &lt;=&gt; ↓ 24.0KB/s",
		"Disco: R 1.0KB/s &lt;=&gt; W 2.0KB/s",
		"💾 <b>Almacenamiento</b>",
		"/ 90.0GB/128.0GB =&gt; (70.3%)",
		"🖥️ <b>GPU</b>",
		"• GPU0 RTX 4090",
		"• Util 85 - Mem 1000MiB / 12000MiB",
		"• Temp 65ºC",
		"• Potencia 250W",
		"⏱️ <b>Uptime</b>",
		"• En marcha: 3h 4m 5s",
		"⚠️ <b>Advertencias</b>",
		"• GPU: unavailable",
		"• Disk: slow",
	}

	for _, needle := range mustContain {
		if !strings.Contains(output, needle) {
			t.Fatalf("FormatHTML output missing %q:\n%s", needle, output)
		}
	}
}

func TestFormatHTMLNoGPU(t *testing.T) {
	stats := Stats{
		CPU: CPUStats{Cores: 0},
		Memory: MemoryStats{
			Total: 0,
		},
	}

	output := FormatHTML(stats)
	if !strings.Contains(output, "GPU: no disponible") {
		t.Fatalf("expected fallback GPU message, got: %s", output)
	}
}

func TestNewCollectorDefaults(t *testing.T) {
	c := NewCollector(Options{})
	if got := c.SampleInterval(); got != time.Second {
		t.Fatalf("SampleInterval() = %v, want 1s default", got)
	}

	if len(c.options.DiskTargets) != 1 || c.options.DiskTargets[0] != "/" {
		t.Fatalf("DiskTargets default = %v, want [/]", c.options.DiskTargets)
	}

	custom := NewCollector(Options{SampleInterval: 5 * time.Second, DiskTargets: []string{"/var"}})
	if got := custom.SampleInterval(); got != 5*time.Second {
		t.Fatalf("SampleInterval custom = %v, want 5s", got)
	}
	if len(custom.options.DiskTargets) != 1 || custom.options.DiskTargets[0] != "/var" {
		t.Fatalf("DiskTargets custom = %v, want [/var]", custom.options.DiskTargets)
	}
}
