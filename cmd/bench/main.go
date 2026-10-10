// Command bench measures end-to-end latency of POST /v1/keywords.
//
// It starts server as a child process with GOMAXPROCS=1 (optionally
// pinned to one CPU with taskset), waits for /healthz, sends warm-up
// requests, then sends the measured requests round-robin over the article
// files and reports latency percentiles, throughput and the server's peak
// resident memory (VmHWM from /proc/<pid>/status, Linux only). With -cgroup
// the server is moved into a prepared cgroup v2 (for example 1 CPU and
// 512 MiB) before warm-up, and the cgroup's peak memory, OOM kills and CPU
// throttling are reported too.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type config struct {
	server      string
	cpu         string
	articles    string
	requests    int
	warmup      int
	concurrency int
	limit       int
	cgroup      string
}

type report struct {
	Host           string        `json:"host"`
	GoVersion      string        `json:"go_version"`
	ServerCPU      string        `json:"server_cpu_pin"`
	Articles       int           `json:"articles"`
	MeanWords      int           `json:"mean_words_per_article"`
	MeanBytes      int           `json:"mean_bytes_per_article"`
	Warmup         int           `json:"warmup_requests"`
	Requests       int           `json:"requests"`
	Errors         int64         `json:"errors"`
	Concurrency    int           `json:"concurrency"`
	P50Ms          float64       `json:"p50_ms"`
	P95Ms          float64       `json:"p95_ms"`
	P99Ms          float64       `json:"p99_ms"`
	MaxMs          float64       `json:"max_ms"`
	MeanMs         float64       `json:"mean_ms"`
	ThroughputRPS  float64       `json:"throughput_rps"`
	ServerPeakRSS  string        `json:"server_peak_rss"`
	ServerFinalRSS string        `json:"server_final_rss"`
	Cgroup         *cgroupReport `json:"cgroup,omitempty"`
}

func main() {
	var c config
	flag.StringVar(&c.server, "server", "./server", "path to the server binary")
	flag.StringVar(&c.cpu, "cpu", "0", "CPU list for taskset; empty to disable pinning")
	flag.StringVar(&c.articles, "articles", "examples", "directory of <lang>/*.txt texts")
	flag.IntVar(&c.requests, "n", 2000, "measured requests")
	flag.IntVar(&c.warmup, "warmup", 200, "warm-up requests (not measured)")
	flag.IntVar(&c.concurrency, "c", 1, "concurrent clients")
	flag.IntVar(&c.limit, "limit", 5, "limit field of each request")
	flag.StringVar(&c.cgroup, "cgroup", "", "cgroup v2 directory to move the server into before warm-up")
	flag.Parse()

	r, err := run(c)
	if err != nil {
		log.Fatal(err)
	}
	out, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(out))
}

func run(c config) (report, error) {
	bodies, words, size, err := loadBodies(c.articles, c.limit)
	if err != nil {
		return report{}, err
	}
	addr, err := freeAddr()
	if err != nil {
		return report{}, err
	}
	cmd := serverCommand(c, addr)
	if err := cmd.Start(); err != nil {
		return report{}, fmt.Errorf("start server: %w", err)
	}
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	}()
	if c.cgroup != "" {
		pid := []byte(strconv.Itoa(cmd.Process.Pid))
		if err := os.WriteFile(filepath.Join(c.cgroup, "cgroup.procs"), pid, 0); err != nil {
			return report{}, fmt.Errorf("move server into cgroup: %w", err)
		}
	}

	base := "http://" + addr
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: c.concurrency, DisableCompression: true},
	}
	if err := waitHealthy(client, base); err != nil {
		return report{}, err
	}

	var errs atomic.Int64
	send := func(body []byte) time.Duration {
		start := time.Now()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, base+"/v1/keywords", bytes.NewReader(body))
		if err != nil {
			errs.Add(1)

			return time.Since(start)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			errs.Add(1)

			return time.Since(start)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			errs.Add(1)
		}

		return time.Since(start)
	}

	for i := range c.warmup {
		send(bodies[i%len(bodies)])
	}
	if errs.Load() > 0 {
		return report{}, fmt.Errorf("%d warm-up requests failed", errs.Load())
	}

	latencies := make([]time.Duration, c.requests)
	var next atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()
	for range c.concurrency {
		wg.Go(func() {
			for {
				i := int(next.Add(1)) - 1
				if i >= c.requests {
					return
				}
				latencies[i] = send(bodies[i%len(bodies)])
			}
		})
	}
	wg.Wait()
	elapsed := time.Since(start)

	peak, final := memStatus(cmd.Process.Pid)
	var cg *cgroupReport
	if c.cgroup != "" {
		cg = readCgroup(c.cgroup)
	}
	slices.Sort(latencies)
	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	host, _ := os.Hostname()

	return report{
		Host:           fmt.Sprintf("%s, %s/%s, %d logical CPUs, %s", host, runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), cpuModel()),
		GoVersion:      runtime.Version(),
		ServerCPU:      orDefault(c.cpu, "not pinned"),
		Articles:       len(bodies),
		MeanWords:      words / len(bodies),
		MeanBytes:      size / len(bodies),
		Warmup:         c.warmup,
		Requests:       c.requests,
		Errors:         errs.Load(),
		Concurrency:    c.concurrency,
		P50Ms:          ms(percentile(latencies, 0.50)),
		P95Ms:          ms(percentile(latencies, 0.95)),
		P99Ms:          ms(percentile(latencies, 0.99)),
		MaxMs:          ms(latencies[len(latencies)-1]),
		MeanMs:         ms(total / time.Duration(len(latencies))),
		ThroughputRPS:  float64(c.requests) / elapsed.Seconds(),
		ServerPeakRSS:  peak,
		ServerFinalRSS: final,
		Cgroup:         cg,
	}, nil
}

// cgroupReport describes the cgroup v2 limits the server ran under and what
// it consumed. Values are copied verbatim from the cgroup files.
type cgroupReport struct {
	Path          string `json:"path"`
	MemoryMax     string `json:"memory_max"`
	MemoryPeak    string `json:"memory_peak_bytes"`
	OOMKills      string `json:"oom_kills"`
	CPUMax        string `json:"cpu_max"`
	CPUSet        string `json:"cpuset_cpus"`
	NrThrottled   string `json:"cpu_nr_throttled"`
	ThrottledUsec string `json:"cpu_throttled_usec"`
}

func readCgroup(dir string) *cgroupReport {
	read := func(name string) string {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "unavailable"
		}

		return strings.TrimSpace(string(raw))
	}
	field := func(name, key string) string {
		for line := range strings.Lines(read(name)) {
			if k, v, ok := strings.Cut(strings.TrimSpace(line), " "); ok && k == key {
				return v
			}
		}

		return "unavailable"
	}

	return &cgroupReport{
		Path:          dir,
		MemoryMax:     read("memory.max"),
		MemoryPeak:    read("memory.peak"),
		OOMKills:      field("memory.events", "oom_kill"),
		CPUMax:        read("cpu.max"),
		CPUSet:        read("cpuset.cpus"),
		NrThrottled:   field("cpu.stat", "nr_throttled"),
		ThrottledUsec: field("cpu.stat", "throttled_usec"),
	}
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func serverCommand(c config, addr string) *exec.Cmd {
	args := []string{c.server, "-addr", addr}
	if c.cpu != "" {
		args = append([]string{"taskset", "-c", c.cpu}, args...)
	}
	cmd := exec.CommandContext(context.Background(), args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "GOMAXPROCS=1")
	cmd.Stderr = os.Stderr
	return cmd
}

func loadBodies(dir string, limit int) (bodies [][]byte, words, size int, err error) {
	files, err := filepath.Glob(filepath.Join(dir, "*", "*.txt"))
	if err != nil || len(files) == 0 {
		return nil, 0, 0, fmt.Errorf("no articles in %s", dir)
	}
	slices.Sort(files)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, 0, 0, err
		}
		language := filepath.Base(filepath.Dir(f))
		b, _ := json.Marshal(map[string]any{"text": string(raw), "language": language, "limit": limit})
		bodies = append(bodies, b)
		words += len(strings.Fields(string(raw)))
		size += len(raw)
	}
	return bodies, words, size, nil
}

func freeAddr() (string, error) {
	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String(), nil
}

func waitHealthy(client *http.Client, base string) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/healthz", nil)
		if reqErr != nil {
			time.Sleep(50 * time.Millisecond)

			continue
		}
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server at %s did not become healthy", base)
}

// memStatus returns VmHWM (peak RSS) and VmRSS of pid. With taskset the
// direct child is the server itself, because taskset execs the command.
func memStatus(pid int) (peak, current string) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return "unavailable", "unavailable"
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), ":")
		if !ok {
			continue
		}
		switch k {
		case "VmHWM":
			peak = strings.TrimSpace(v)
		case "VmRSS":
			current = strings.TrimSpace(v)
		}
	}
	return peak, current
}

func cpuModel() string {
	raw, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown CPU"
	}
	for line := range strings.Lines(string(raw)) {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
			return strings.TrimSpace(v)
		}
	}
	return "unknown CPU"
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	i := int(float64(len(sorted))*p+0.5) - 1
	return sorted[min(max(i, 0), len(sorted)-1)]
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}
