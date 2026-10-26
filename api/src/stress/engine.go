package main

import (
	"context"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// maxCapturedBody bounds how much of each response is kept in memory. The body
// is drained afterwards so connections can be reused.
const maxCapturedBody = 256 << 10

type workerStats struct {
	latencies       []time.Duration
	statuses        map[int]int64
	transportErrors int64
	buildErrors     int64
	bytes           int64
}

// Run executes the load test and returns its aggregated report.
//
// Each worker executes requests in a closed loop. When -rate is set, a shared
// ticker paces the whole run instead.
func Run(ctx context.Context, opts *Options, client *http.Client, scenario Scenario) (*Report, error) {
	runCtx := ctx
	if opts.Duration > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Duration)
		defer cancel()
	}

	var tokens <-chan time.Time
	if opts.Rate > 0 {
		interval := time.Duration(float64(time.Second) / opts.Rate)
		if interval < time.Nanosecond {
			interval = time.Nanosecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		tokens = ticker.C
	}

	var (
		nextRequest atomic.Int64
		completed   atomic.Int64
		wg          sync.WaitGroup
	)

	done := make(chan struct{})
	go logProgress(done, &completed)

	stats := make([]workerStats, opts.Concurrency)

	startedAt := time.Now()
	for id := 0; id < opts.Concurrency; id++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			stats[workerID] = runWorker(runCtx, opts, client, scenario, workerID, &nextRequest, tokens, &completed)
		}(id)
	}
	wg.Wait()
	elapsed := time.Since(startedAt)

	close(done)

	return buildReport(opts, scenario.Name(), elapsed, stats), nil
}

func runWorker(
	ctx context.Context,
	opts *Options,
	client *http.Client,
	scenario Scenario,
	workerID int,
	nextRequest *atomic.Int64,
	tokens <-chan time.Time,
	completed *atomic.Int64,
) workerStats {
	result := workerStats{
		statuses:  make(map[int]int64, 8),
		latencies: make([]time.Duration, 0, estimateCapacity(opts)),
	}
	state := &WorkerState{ID: workerID}

	for {
		if ctx.Err() != nil {
			return result
		}

		if opts.Requests > 0 && nextRequest.Add(1) > opts.Requests {
			return result
		}

		if tokens != nil {
			select {
			case <-ctx.Done():
				return result
			case <-tokens:
			}
		}

		req, err := scenario.Next(ctx, state)
		if err != nil {
			if ctx.Err() != nil {
				return result
			}
			result.buildErrors++
			time.Sleep(5 * time.Millisecond)
			continue
		}

		if opts.DistributeIP {
			req.Header.Set("X-Forwarded-For", nextFakeIP())
		}

		start := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(start)

		if err != nil {
			if ctx.Err() != nil {
				return result
			}
			result.transportErrors++
			result.latencies = append(result.latencies, latency)
			completed.Add(1)
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxCapturedBody))
		drained, _ := io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if readErr == nil {
			result.bytes += int64(len(body)) + drained
		}

		result.statuses[resp.StatusCode]++
		result.latencies = append(result.latencies, latency)
		completed.Add(1)

		if hook, ok := scenario.(AfterHook); ok {
			hook.After(state, resp.StatusCode, body)
		}
	}
}

func logProgress(done <-chan struct{}, completed *atomic.Int64) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			log.Printf("progresso: %d requisições concluídas", completed.Load())
		}
	}
}

func buildReport(opts *Options, scenarioName string, elapsed time.Duration, stats []workerStats) *Report {
	report := &Report{
		BaseURL:      opts.BaseURL,
		Scenario:     scenarioName,
		Concurrency:  opts.Concurrency,
		Elapsed:      elapsed,
		StatusCounts: make(map[int]int64, 8),
	}

	latencies := make([]time.Duration, 0, 1024)
	var total time.Duration

	for _, stat := range stats {
		report.TransportErrors += stat.transportErrors
		report.BuildErrors += stat.buildErrors
		report.Bytes += stat.bytes

		for status, count := range stat.statuses {
			report.StatusCounts[status] += count
		}

		latencies = append(latencies, stat.latencies...)
		for _, latency := range stat.latencies {
			total += latency
		}
	}

	report.Requests = report.TransportErrors
	for _, count := range report.StatusCounts {
		report.Requests += count
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

		report.Latency = LatencyStats{
			Min: latencies[0],
			Max: latencies[len(latencies)-1],
			Avg: total / time.Duration(len(latencies)),
			P50: percentile(latencies, 0.50),
			P90: percentile(latencies, 0.90),
			P95: percentile(latencies, 0.95),
			P99: percentile(latencies, 0.99),
		}
	}

	return report
}

// percentile uses the nearest-rank method over an ascending slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}

	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func estimateCapacity(opts *Options) int {
	if opts.Requests <= 0 {
		return 1024
	}

	perWorker := opts.Requests/int64(opts.Concurrency) + 1
	if perWorker > 1_000_000 {
		return 1_000_000
	}
	return int(perWorker)
}
