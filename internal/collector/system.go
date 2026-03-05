package collector

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kostas/homedash/internal/config"
)

type cpuSample struct {
	idle  uint64
	total uint64
}

type netSample struct {
	rxBytes uint64
	txBytes uint64
	at      time.Time
}

// prevCPU and prevNet are intentionally package-level mutable samples.
// CollectSystem is only called from the Bubble Tea update loop on a single
// goroutine, so this state has single-goroutine ownership. Callers must not
// invoke CollectSystem concurrently.
var (
	prevCPU cpuSample
	prevNet netSample
)

func CollectSystem(disks []config.Disk) (SystemData, error) {
	var data SystemData
	data.CollectedAt = time.Now()

	// Hostname
	hostname, err := os.Hostname()
	if err == nil {
		data.Hostname = hostname
	}

	// Uptime
	if raw, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) >= 1 {
			if secs, err := strconv.ParseFloat(fields[0], 64); err == nil {
				data.Uptime = time.Duration(secs * float64(time.Second))
			}
		}
	}

	// CPU
	if raw, err := os.ReadFile("/proc/stat"); err == nil {
		lines := strings.Split(string(raw), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "cpu ") {
				fields := strings.Fields(line)
				if len(fields) >= 8 {
					var values [8]uint64
					for i := 1; i <= 7; i++ {
						values[i-1], _ = strconv.ParseUint(fields[i], 10, 64)
					}
					// user, nice, system, idle, iowait, irq, softirq
					idle := values[3] + values[4]
					total := values[0] + values[1] + values[2] + values[3] + values[4] + values[5] + values[6]

					if prevCPU.total > 0 {
						idleDelta := float64(idle - prevCPU.idle)
						totalDelta := float64(total - prevCPU.total)
						if totalDelta > 0 {
							data.CPUPercent = (1.0 - idleDelta/totalDelta) * 100
						}
					}
					prevCPU = cpuSample{idle: idle, total: total}
				}
				break
			}
		}

		// Count CPU cores
		for _, line := range lines {
			if strings.HasPrefix(line, "cpu") && !strings.HasPrefix(line, "cpu ") {
				data.CPUCount++
			}
		}
	}

	// Memory
	if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
		memInfo := parseMemInfo(string(raw))
		total := memInfo["MemTotal"]
		available := memInfo["MemAvailable"]
		data.MemTotal = total * 1024 // kB to bytes
		data.MemUsed = (total - available) * 1024
		if total > 0 {
			data.MemPercent = float64(total-available) / float64(total) * 100
		}
	}

	// Load average
	if raw, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) >= 3 {
			data.LoadAvg[0], _ = strconv.ParseFloat(fields[0], 64)
			data.LoadAvg[1], _ = strconv.ParseFloat(fields[1], 64)
			data.LoadAvg[2], _ = strconv.ParseFloat(fields[2], 64)
		}
	}

	// Disks
	for _, disk := range disks {
		d, err := diskInfo(disk)
		if err != nil {
			data.Warnings = append(data.Warnings,
				fmt.Sprintf("disk %s: %v", disk.Path, err))
			continue
		}
		data.Disks = append(data.Disks, d)
	}

	// Network
	if raw, err := os.ReadFile("/proc/net/dev"); err == nil {
		rxTotal, txTotal := parseNetDev(string(raw))
		now := time.Now()
		if prevNet.at.IsZero() {
			prevNet = netSample{rxBytes: rxTotal, txBytes: txTotal, at: now}
		} else {
			elapsed := now.Sub(prevNet.at).Seconds()
			if elapsed > 0 {
				var rxDelta, txDelta uint64
				if rxTotal >= prevNet.rxBytes {
					rxDelta = rxTotal - prevNet.rxBytes
				}
				if txTotal >= prevNet.txBytes {
					txDelta = txTotal - prevNet.txBytes
				}
				data.NetRxRate = float64(rxDelta) / elapsed
				data.NetTxRate = float64(txDelta) / elapsed
			}
			prevNet = netSample{rxBytes: rxTotal, txBytes: txTotal, at: now}
		}
	}

	return data, nil
}

// parseNetDev parses /proc/net/dev and returns aggregate rx/tx bytes
// for physical interfaces (skips lo, docker*, veth*, br-*).
func parseNetDev(content string) (rxTotal, txTotal uint64) {
	for _, line := range strings.Split(content, "\n") {
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:colonIdx])

		// Skip virtual interfaces.
		if iface == "lo" ||
			strings.HasPrefix(iface, "docker") ||
			strings.HasPrefix(iface, "veth") ||
			strings.HasPrefix(iface, "br-") {
			continue
		}

		fields := strings.Fields(line[colonIdx+1:])
		if len(fields) < 10 {
			continue
		}

		rx, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		tx, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			continue
		}
		rxTotal += rx
		txTotal += tx
	}
	return rxTotal, txTotal
}

func parseMemInfo(content string) map[string]uint64 {
	result := make(map[string]uint64)
	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])
		valStr = strings.TrimSuffix(valStr, " kB")
		valStr = strings.TrimSpace(valStr)
		if v, err := strconv.ParseUint(valStr, 10, 64); err == nil {
			result[key] = v
		}
	}
	return result
}

func diskInfo(disk config.Disk) (DiskInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(disk.Path, &stat); err != nil {
		return DiskInfo{}, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}

	label := strings.TrimSpace(disk.Label)
	if label == "" {
		label = disk.Path
	}

	return DiskInfo{
		Mount:   label,
		Total:   total,
		Used:    used,
		Percent: pct,
	}, nil
}

// FormatBytes formats bytes into human-readable string.
func FormatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.1fT", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1fG", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.0fM", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.0fK", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
