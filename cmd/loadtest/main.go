// loadtest hammers the transfer endpoint with unique idempotency keys to
// measure real end-to-end throughput including PostgreSQL writes.
//
// Usage:
//
//	go run ./cmd/loadtest/ -from=<acc1> -to=<acc2> -concurrency=50 -duration=30s
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	from := flag.String("from", "", "source account ID (required)")
	to := flag.String("to", "", "destination account ID (required)")
	baseURL := flag.String("url", "http://localhost:8080", "base URL of the server")
	concurrency := flag.Int("concurrency", 50, "number of parallel goroutines")
	duration := flag.Duration("duration", 30*time.Second, "how long to run")
	amount := flag.Int64("amount", 1, "transfer amount per request (smallest currency unit)")
	flag.Parse()

	if *from == "" || *to == "" {
		fmt.Fprintln(os.Stderr, "error: -from and -to are required")
		flag.Usage()
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	endpoint := *baseURL + "/transfer"

	var (
		successes int64
		failures  int64
	)

	// Each worker collects its own latency samples to avoid channel overflow.
	workerLatencies := make([][]float64, *concurrency)
	deadline := time.Now().Add(*duration)

	var wg sync.WaitGroup
	fmt.Printf("Starting load test: %d goroutines for %s against %s\n\n",
		*concurrency, *duration, endpoint)

	start := time.Now()
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		workerLatencies[i] = make([]float64, 0, 1024)
		go func(workerID int) {
			defer wg.Done()
			var localN int64
			for time.Now().Before(deadline) {
				key := fmt.Sprintf("load-%d-%d", workerID, localN)
				localN++

				body, _ := json.Marshal(map[string]any{
					"from":     *from,
					"to":       *to,
					"amount":   *amount,
					"currency": "INR",
				})

				req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", key)

				t0 := time.Now()
				resp, err := client.Do(req)
				lat := time.Since(t0)

				workerLatencies[workerID] = append(workerLatencies[workerID], float64(lat.Milliseconds()))

				if err != nil || resp == nil || resp.StatusCode >= 400 {
					atomic.AddInt64(&failures, 1)
					if resp != nil {
						resp.Body.Close()
					}
					continue
				}
				resp.Body.Close()
				atomic.AddInt64(&successes, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Merge per-worker latency slices.
	var latencies []float64
	for _, wl := range workerLatencies {
		latencies = append(latencies, wl...)
	}
	sort.Float64s(latencies)

	total := successes + failures
	tps := float64(successes) / elapsed.Seconds()

	pct := func(p float64) float64 {
		if len(latencies) == 0 {
			return 0
		}
		idx := int(float64(len(latencies)) * p / 100)
		if idx >= len(latencies) {
			idx = len(latencies) - 1
		}
		return latencies[idx]
	}

	fmt.Printf("Duration:     %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Concurrency:  %d goroutines\n", *concurrency)
	fmt.Printf("Total:        %d requests\n", total)
	fmt.Printf("Successful:   %d\n", successes)
	fmt.Printf("Failed:       %d\n", failures)
	fmt.Printf("\nThroughput:   %.0f req/s\n", tps)
	fmt.Printf("\nLatency (ms):\n")
	fmt.Printf("  p50:  %.1f ms\n", pct(50))
	fmt.Printf("  p90:  %.1f ms\n", pct(90))
	fmt.Printf("  p95:  %.1f ms\n", pct(95))
	fmt.Printf("  p99:  %.1f ms\n", pct(99))
	fmt.Printf("  max:  %.1f ms\n", pct(100))
}
