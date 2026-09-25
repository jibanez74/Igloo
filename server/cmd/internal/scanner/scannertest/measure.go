package scannertest

import (
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// MeasurementLogger reports per-phase wall time for opt-in library
// benchmarks. It keys on the "<library> scan phase" and "<library> scan
// finished" messages every scanner logs. It deliberately does not capture
// entries: a benchmark over a real library logs per-file debug lines, and
// retaining them would show up in the RSS it measures.
type MeasurementLogger struct {
	mu    sync.Mutex
	t     *testing.T
	phase string
	start time.Time
}

func NewMeasurementLogger(t *testing.T) *MeasurementLogger {
	return &MeasurementLogger{t: t}
}

func (l *MeasurementLogger) Info(message string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	phaseEvent := strings.HasSuffix(message, " scan phase") || strings.HasSuffix(message, " scan finished")
	if phaseEvent {
		if !l.start.IsZero() {
			l.t.Logf("phase %s: %s", l.phase, time.Since(l.start))
		}
		l.start = time.Now()
		for i := 0; i+1 < len(args); i += 2 {
			if args[i] == "phase" {
				l.phase, _ = args[i+1].(string)
			}
		}
	}
	l.t.Log(message, args)
}

func (l *MeasurementLogger) Debug(string, ...any)              {}
func (l *MeasurementLogger) Warn(message string, args ...any)  { l.t.Log(message, args) }
func (l *MeasurementLogger) Error(message string, args ...any) { l.t.Log(message, args) }

// MeasureRun times one benchmark pass and logs its heap and process I/O
// around scan, which runs the pass and returns the library's status summary.
func MeasureRun(t *testing.T, run string, scan func() string) {
	t.Helper()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	t.Logf("%s initial I/O: %s", run, ProcessMeasurement("/proc/self/io"))
	summary := scan()
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	t.Logf("%s: elapsed=%s %s heap=%d allocated=%d", run, elapsed, summary, after.HeapAlloc, after.TotalAlloc-before.TotalAlloc)
	t.Logf("%s final I/O: %s; memory: %s", run, ProcessMeasurement("/proc/self/io"), ProcessMeasurement("/proc/self/status"))
}

// ProcessMeasurement summarizes /proc/self/io or /proc/self/status for a
// benchmark log line; other platforms report unavailability.
func ProcessMeasurement(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unavailable on this platform"
	}
	if strings.HasSuffix(path, "/status") {
		lines := make([]string, 0)
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VmRSS:") || strings.HasPrefix(line, "VmHWM:") {
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "; ")
	}
	return strings.ReplaceAll(strings.TrimSpace(string(data)), "\n", "; ")
}
