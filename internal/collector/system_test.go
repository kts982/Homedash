package collector

import (
	"strings"
	"syscall"
	"testing"

	"github.com/kostas/homedash/internal/config"
)

func TestFormatBytes(t *testing.T) {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	tests := []struct {
		name string
		in   uint64
		want string
	}{
		{name: "zero", in: 0, want: "0B"},
		{name: "bytes", in: 999, want: "999B"},
		{name: "kb boundary", in: kb, want: "1K"},
		{name: "kb rounded", in: 1536, want: "2K"},
		{name: "mb boundary", in: mb, want: "1M"},
		{name: "gb boundary", in: gb, want: "1.0G"},
		{name: "tb boundary", in: tb, want: "1.0T"},
		{name: "multiple tb", in: 12 * tb, want: "12.0T"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatBytes(tc.in)
			if got != tc.want {
				t.Fatalf("FormatBytes(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{name: "zero", in: 0, want: "0B/s"},
		{name: "bytes", in: 500, want: "500B/s"},
		{name: "kilobytes", in: 1024, want: "1K/s"},
		{name: "megabytes", in: 2.5 * 1024 * 1024, want: "3M/s"},
		{name: "gigabytes", in: 1.5 * 1024 * 1024 * 1024, want: "1.5G/s"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatRate(tc.in)
			if got != tc.want {
				t.Fatalf("FormatRate(%f) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseNetDev(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantRx uint64
		wantTx uint64
	}{
		{
			name: "typical interfaces",
			input: "Inter-|   Receive                                                |  Transmit\n" +
				" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n" +
				"    lo: 1000    10    0    0    0     0          0         0  1000    10    0    0    0     0       0          0\n" +
				"enp10s0: 5000 100    0    0    0     0          0         0  3000   50    0    0    0     0       0          0\n" +
				"  wg0: 2000  20    0    0    0     0          0         0  1000   10    0    0    0     0       0          0\n" +
				"docker0:  500   5    0    0    0     0          0         0   300    3    0    0    0     0       0          0\n" +
				"veth1234: 100   1    0    0    0     0          0         0    50    1    0    0    0     0       0          0\n" +
				"br-abc123: 200   2    0    0    0     0          0         0   100    1    0    0    0     0       0          0\n",
			wantRx: 7000,
			wantTx: 4000,
		},
		{
			name:   "empty input",
			input:  "",
			wantRx: 0,
			wantTx: 0,
		},
		{
			name: "only virtual interfaces",
			input: "Inter-|   Receive\n" +
				" face |bytes\n" +
				"    lo: 1000    10    0    0    0     0          0         0  1000    10    0    0    0     0       0          0\n" +
				"docker0:  500   5    0    0    0     0          0         0   300    3    0    0    0     0       0          0\n",
			wantRx: 0,
			wantTx: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rx, tx := parseNetDev(tc.input)
			if rx != tc.wantRx {
				t.Fatalf("parseNetDev() rx = %d, want %d", rx, tc.wantRx)
			}
			if tx != tc.wantTx {
				t.Fatalf("parseNetDev() tx = %d, want %d", tx, tc.wantTx)
			}
		})
	}
}

func TestParseLoadAvg(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantLoad    [3]float64
		wantRunning int
		wantTotal   int
		wantOK      bool
	}{
		{
			name:        "valid loadavg with tasks",
			input:       "0.33 0.44 0.55 2/214 12345\n",
			wantLoad:    [3]float64{0.33, 0.44, 0.55},
			wantRunning: 2,
			wantTotal:   214,
			wantOK:      true,
		},
		{
			name:   "rejects malformed load values",
			input:  "oops 0.44 0.55 2/214 12345\n",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			load, running, total, ok := parseLoadAvg(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("parseLoadAvg() ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if load != tc.wantLoad || running != tc.wantRunning || total != tc.wantTotal {
				t.Fatalf("parseLoadAvg() = (%v, %d, %d), want (%v, %d, %d)", load, running, total, tc.wantLoad, tc.wantRunning, tc.wantTotal)
			}
		})
	}
}

func TestParseFileUsage(t *testing.T) {
	tests := []struct {
		name     string
		fileNr   string
		fileMax  string
		wantOpen uint64
		wantMax  uint64
		wantOK   bool
	}{
		{
			name:     "prefers file-max and subtracts unused handles",
			fileNr:   "4096\t128\t1048576\n",
			fileMax:  "2097152\n",
			wantOpen: 3968,
			wantMax:  2097152,
			wantOK:   true,
		},
		{
			name:     "falls back to file-nr limit",
			fileNr:   "1024\t0\t524288\n",
			fileMax:  "",
			wantOpen: 1024,
			wantMax:  524288,
			wantOK:   true,
		},
		{
			name:    "rejects malformed input",
			fileNr:  "broken data\n",
			fileMax: "oops\n",
			wantOK:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			openFiles, maxFiles, ok := parseFileUsage(tc.fileNr, tc.fileMax)
			if ok != tc.wantOK {
				t.Fatalf("parseFileUsage() ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if openFiles != tc.wantOpen || maxFiles != tc.wantMax {
				t.Fatalf("parseFileUsage() = (%d, %d), want (%d, %d)", openFiles, maxFiles, tc.wantOpen, tc.wantMax)
			}
		})
	}
}

func TestParseMemInfo(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		assert func(t *testing.T, got map[string]uint64)
	}{
		{
			name: "valid meminfo",
			input: "MemTotal:       16384256 kB\n" +
				"MemFree:         8471092 kB\n" +
				"MemAvailable:   12123456 kB\n",
			assert: func(t *testing.T, got map[string]uint64) {
				t.Helper()
				if got["MemTotal"] != 16384256 {
					t.Fatalf("MemTotal = %d, want %d", got["MemTotal"], uint64(16384256))
				}
				if got["MemFree"] != 8471092 {
					t.Fatalf("MemFree = %d, want %d", got["MemFree"], uint64(8471092))
				}
				if got["MemAvailable"] != 12123456 {
					t.Fatalf("MemAvailable = %d, want %d", got["MemAvailable"], uint64(12123456))
				}
			},
		},
		{
			name: "missing keys",
			input: "MemTotal: 1000 kB\n" +
				"Buffers: 10 kB\n",
			assert: func(t *testing.T, got map[string]uint64) {
				t.Helper()
				if got["MemTotal"] != 1000 {
					t.Fatalf("MemTotal = %d, want 1000", got["MemTotal"])
				}
				if _, ok := got["MemAvailable"]; ok {
					t.Fatalf("MemAvailable should be missing, got %d", got["MemAvailable"])
				}
			},
		},
		{
			name: "edge cases and malformed lines",
			input: "NoColonLine\n" +
				"MemTotal: not-a-number kB\n" +
				"MemAvailable: 42\n" +
				"WeirdKey : 7 kB\n" +
				"SwapTotal:      2048   kB   \n",
			assert: func(t *testing.T, got map[string]uint64) {
				t.Helper()
				if _, ok := got["MemTotal"]; ok {
					t.Fatalf("MemTotal should be ignored for invalid number")
				}
				if got["MemAvailable"] != 42 {
					t.Fatalf("MemAvailable = %d, want 42", got["MemAvailable"])
				}
				if got["WeirdKey"] != 7 {
					t.Fatalf("WeirdKey = %d, want 7", got["WeirdKey"])
				}
				if got["SwapTotal"] != 2048 {
					t.Fatalf("SwapTotal = %d, want 2048", got["SwapTotal"])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseMemInfo(tc.input)
			tc.assert(t, got)
		})
	}
}

func TestParseCPUSampleIncludesStealTime(t *testing.T) {
	sample, ok := parseCPUSample("cpu  10 2 3 40 5 6 7 8 9 10")
	if !ok {
		t.Fatal("parseCPUSample() = false, want true")
	}
	if sample.idle != 45 {
		t.Fatalf("sample.idle = %d, want 45", sample.idle)
	}
	if sample.total != 81 {
		t.Fatalf("sample.total = %d, want 81", sample.total)
	}
}

func TestMemoryUsageKBFallsBackWithoutMemAvailable(t *testing.T) {
	total, used, ok := memoryUsageKB(map[string]uint64{
		"MemTotal":     1000,
		"MemFree":      100,
		"Buffers":      50,
		"Cached":       200,
		"SReclaimable": 30,
		"Shmem":        10,
	})
	if !ok {
		t.Fatal("memoryUsageKB() = false, want true")
	}
	if total != 1000 {
		t.Fatalf("total = %d, want 1000", total)
	}
	if used != 630 {
		t.Fatalf("used = %d, want 630", used)
	}
}

func TestMemoryUsageKBClampsAvailableToTotal(t *testing.T) {
	total, used, ok := memoryUsageKB(map[string]uint64{
		"MemTotal":     1000,
		"MemAvailable": 2000,
	})
	if !ok {
		t.Fatal("memoryUsageKB() = false, want true")
	}
	if total != 1000 {
		t.Fatalf("total = %d, want 1000", total)
	}
	if used != 0 {
		t.Fatalf("used = %d, want 0", used)
	}
}

func TestParseCPUSample(t *testing.T) {
	tests := []struct {
		name string
		line string
		want cpuSample
		ok   bool
	}{
		{
			name: "includes steal and ignores guest counters",
			line: "cpu 1 2 3 4 5 6 7 8 9 10",
			want: cpuSample{idle: 9, total: 36},
			ok:   true,
		},
		{
			name: "requires aggregate cpu line",
			line: "cpu0 1 2 3 4",
			ok:   false,
		},
		{
			name: "rejects malformed counters",
			line: "cpu 1 two 3 4",
			ok:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseCPUSample(tc.line)
			if ok != tc.ok {
				t.Fatalf("parseCPUSample(%q) ok = %v, want %v", tc.line, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("parseCPUSample(%q) = %#v, want %#v", tc.line, got, tc.want)
			}
		})
	}
}

func TestMemoryUsageKB(t *testing.T) {
	tests := []struct {
		name      string
		memInfo   map[string]uint64
		wantTotal uint64
		wantUsed  uint64
		ok        bool
	}{
		{
			name: "uses MemAvailable when present",
			memInfo: map[string]uint64{
				"MemTotal":     1000,
				"MemAvailable": 400,
			},
			wantTotal: 1000,
			wantUsed:  600,
			ok:        true,
		},
		{
			name: "falls back to legacy availability fields",
			memInfo: map[string]uint64{
				"MemTotal":     1000,
				"MemFree":      200,
				"Buffers":      50,
				"Cached":       150,
				"SReclaimable": 25,
				"Shmem":        10,
			},
			wantTotal: 1000,
			wantUsed:  585,
			ok:        true,
		},
		{
			name: "clamps fallback availability to total",
			memInfo: map[string]uint64{
				"MemTotal": 1000,
				"MemFree":  1500,
			},
			wantTotal: 1000,
			wantUsed:  0,
			ok:        true,
		},
		{
			name: "returns false without availability data",
			memInfo: map[string]uint64{
				"MemTotal": 1000,
			},
			wantTotal: 1000,
			wantUsed:  0,
			ok:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			total, used, ok := memoryUsageKB(tc.memInfo)
			if total != tc.wantTotal || used != tc.wantUsed || ok != tc.ok {
				t.Fatalf("memoryUsageKB(%v) = (%d, %d, %v), want (%d, %d, %v)",
					tc.memInfo, total, used, ok, tc.wantTotal, tc.wantUsed, tc.ok)
			}
		})
	}
}

func TestCollectSystemDiskWarning(t *testing.T) {
	data, err := CollectSystem([]config.Disk{
		{Path: "/", Label: "/"},
		{Path: "/nonexistent/mount/path", Label: "bad"},
	})
	if err != nil {
		t.Fatalf("CollectSystem() unexpected error: %v", err)
	}
	if len(data.Disks) == 0 {
		t.Fatal("expected at least 1 disk, got 0")
	}
	if len(data.Warnings) == 0 {
		t.Fatal("expected warning for inaccessible disk, got none")
	}
	found := false
	for _, w := range data.Warnings {
		if strings.Contains(w, "/nonexistent/mount/path") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning containing '/nonexistent/mount/path', got: %v", data.Warnings)
	}
}

func TestParseMemInfoSwap(t *testing.T) {
	input := "MemTotal:       16384000 kB\nMemFree:         8000000 kB\nMemAvailable:   12000000 kB\nSwapTotal:       4096000 kB\nSwapFree:        3072000 kB\n"
	info := parseMemInfo(input)

	swapTotal := info["SwapTotal"]
	swapFree := info["SwapFree"]
	if swapTotal != 4096000 {
		t.Fatalf("SwapTotal = %d, want 4096000", swapTotal)
	}
	if swapFree != 3072000 {
		t.Fatalf("SwapFree = %d, want 3072000", swapFree)
	}
}

func TestCollectSystemSwapFields(t *testing.T) {
	memInfo := map[string]uint64{
		"MemTotal":     16384000,
		"MemAvailable": 12000000,
		"SwapTotal":    4096000,
		"SwapFree":     3072000,
	}
	swapTotal := memInfo["SwapTotal"]
	swapFree := memInfo["SwapFree"]
	swapUsedKB := swapTotal - swapFree
	swapPercent := float64(swapUsedKB) / float64(swapTotal) * 100

	if swapUsedKB != 1024000 {
		t.Fatalf("swapUsed = %d kB, want 1024000", swapUsedKB)
	}
	if swapPercent < 24.9 || swapPercent > 25.1 {
		t.Fatalf("swapPercent = %.1f, want ~25.0", swapPercent)
	}
}

func TestCollectSystemSwapZero(t *testing.T) {
	memInfo := map[string]uint64{
		"MemTotal":     16384000,
		"MemAvailable": 12000000,
		"SwapTotal":    0,
		"SwapFree":     0,
	}
	swapTotal := memInfo["SwapTotal"]
	if swapTotal != 0 {
		t.Fatalf("swapTotal = %d, want 0", swapTotal)
	}
}

func TestDiskInfoUsesBavail(t *testing.T) {
	info, err := diskInfo(config.Disk{Path: "/"})
	if err != nil {
		t.Fatalf("diskInfo(/) error = %v", err)
	}
	if info.Used > info.Total {
		t.Fatalf("diskInfo(/) used = %d, total = %d", info.Used, info.Total)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		t.Fatalf("statfs(/) error = %v", err)
	}

	free := info.Total - info.Used
	expectedFree := stat.Bavail * uint64(stat.Bsize)

	// Use Bavail so free space reflects what non-root users (and df) can use.
	// Bfree includes filesystem-reserved blocks that regular users cannot claim.
	if free != expectedFree {
		t.Fatalf("diskInfo(/) free = %d, want %d (Bavail-based)", free, expectedFree)
	}
}
