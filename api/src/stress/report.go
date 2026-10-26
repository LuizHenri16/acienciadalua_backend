package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Report is the aggregated result of a run.
type Report struct {
	BaseURL     string
	Scenario    string
	Concurrency int

	Elapsed  time.Duration
	Requests int64
	Bytes    int64

	StatusCounts map[int]int64

	TransportErrors int64
	BuildErrors     int64

	Latency LatencyStats
}

// LatencyStats is the latency summary, in the same units used internally.
type LatencyStats struct {
	Min time.Duration
	Max time.Duration
	Avg time.Duration
	P50 time.Duration
	P90 time.Duration
	P95 time.Duration
	P99 time.Duration
}

// Throughput is the number of completed requests per second.
func (r *Report) Throughput() float64 {
	if r.Elapsed <= 0 {
		return 0
	}
	return float64(r.Requests) / r.Elapsed.Seconds()
}

// SuccessRate is the share of responses in the 2xx/3xx range.
func (r *Report) SuccessRate() float64 {
	if r.Requests == 0 {
		return 0
	}

	success := int64(0)
	for status, count := range r.StatusCounts {
		if status >= 200 && status < 400 {
			success += count
		}
	}
	return float64(success) / float64(r.Requests)
}

// ErrorRate counts transport failures plus 5xx responses. 4xx responses are
// client/validation outcomes and do not count as errors.
func (r *Report) ErrorRate() float64 {
	if r.Requests == 0 {
		return 0
	}

	failures := r.TransportErrors
	for status, count := range r.StatusCounts {
		if status >= 500 {
			failures += count
		}
	}
	return float64(failures) / float64(r.Requests)
}

func (r *Report) serverErrors() int64 {
	var total int64
	for status, count := range r.StatusCounts {
		if status >= 500 {
			total += count
		}
	}
	return total
}

func printReport(out io.Writer, r *Report) {
	fmt.Fprintln(out, "A Ciência da Lua — teste de estresse")
	fmt.Fprintln(out)

	field(out, "Base URL", r.BaseURL)
	field(out, "Cenário", r.Scenario)
	field(out, "Concorrência", strconv.Itoa(r.Concurrency))
	field(out, "Requisições", strconv.FormatInt(r.Requests, 10))
	field(out, "Duração", fmt.Sprintf("%.2fs", r.Elapsed.Seconds()))
	field(out, "Throughput", fmt.Sprintf("%.1f req/s", r.Throughput()))
	field(out, "Bytes recebidos", humanBytes(r.Bytes))
	field(out, "Códigos HTTP", formatStatuses(r.StatusCounts))
	field(out, "Erros de transporte", strconv.FormatInt(r.TransportErrors, 10))

	if r.BuildErrors > 0 {
		field(out, "Erros ao montar request", strconv.FormatInt(r.BuildErrors, 10))
	}

	field(out, "Latência", formatLatency(r.Latency))
	field(out, "Taxa de sucesso", fmt.Sprintf("%.2f%% (2xx/3xx)", r.SuccessRate()*100))
	field(out, "Taxa de erro", fmt.Sprintf("%.2f%% (transporte + 5xx)", r.ErrorRate()*100))

	if r.StatusCounts[429] > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Aviso: houve respostas 429 (rate limit).")
		fmt.Fprintln(out, "       As rotas de auth aceitam 10 req/min (3/15min no forgot-password).")
		fmt.Fprintln(out, "       Use -distribute-ip para simular IPs distintos em ambiente local/staging.")
	}

	if r.serverErrors() > 0 {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Aviso: %d resposta(s) 5xx — veja os logs da API.\n", r.serverErrors())
	}
}

func field(out io.Writer, label, value string) {
	fmt.Fprintf(out, "%-24s %s\n", label, value)
}

func formatStatuses(statuses map[int]int64) string {
	if len(statuses) == 0 {
		return "-"
	}

	codes := make([]int, 0, len(statuses))
	for code := range statuses {
		codes = append(codes, code)
	}
	sort.Ints(codes)

	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, fmt.Sprintf("%d: %d", code, statuses[code]))
	}
	return strings.Join(parts, " | ")
}

func formatLatency(l LatencyStats) string {
	if l.Max == 0 {
		return "-"
	}

	return fmt.Sprintf(
		"min %s | média %s | p50 %s | p90 %s | p95 %s | p99 %s | máx %s",
		formatDuration(l.Min),
		formatDuration(l.Avg),
		formatDuration(l.P50),
		formatDuration(l.P90),
		formatDuration(l.P95),
		formatDuration(l.P99),
		formatDuration(l.Max),
	)
}

func formatDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0s"
	case d < time.Millisecond:
		return fmt.Sprintf("%.1fµs", float64(d)/float64(time.Microsecond))
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func humanBytes(bytes int64) string {
	const unit = 1024

	switch {
	case bytes < unit:
		return fmt.Sprintf("%d B", bytes)
	case bytes < unit*unit:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/unit)
	default:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/(unit*unit))
	}
}

type jsonReport struct {
	BaseURL         string             `json:"base_url"`
	Scenario        string             `json:"scenario"`
	Concurrency     int                `json:"concurrency"`
	Requests        int64              `json:"requests"`
	DurationSeconds float64            `json:"duration_seconds"`
	ThroughputRPS   float64            `json:"throughput_rps"`
	BytesReceived   int64              `json:"bytes_received"`
	StatusCodes     map[string]int64   `json:"status_codes"`
	TransportErrors int64              `json:"transport_errors"`
	BuildErrors     int64              `json:"build_errors"`
	LatencyMS       map[string]float64 `json:"latency_ms"`
	SuccessRate     float64            `json:"success_rate"`
	ErrorRate       float64            `json:"error_rate"`
}

func printJSON(out io.Writer, r *Report) error {
	statuses := make(map[string]int64, len(r.StatusCounts))
	for code, count := range r.StatusCounts {
		statuses[strconv.Itoa(code)] = count
	}

	payload := jsonReport{
		BaseURL:         r.BaseURL,
		Scenario:        r.Scenario,
		Concurrency:     r.Concurrency,
		Requests:        r.Requests,
		DurationSeconds: r.Elapsed.Seconds(),
		ThroughputRPS:   r.Throughput(),
		BytesReceived:   r.Bytes,
		StatusCodes:     statuses,
		TransportErrors: r.TransportErrors,
		BuildErrors:     r.BuildErrors,
		LatencyMS: map[string]float64{
			"min": durationMS(r.Latency.Min),
			"avg": durationMS(r.Latency.Avg),
			"p50": durationMS(r.Latency.P50),
			"p90": durationMS(r.Latency.P90),
			"p95": durationMS(r.Latency.P95),
			"p99": durationMS(r.Latency.P99),
			"max": durationMS(r.Latency.Max),
		},
		SuccessRate: r.SuccessRate(),
		ErrorRate:   r.ErrorRate(),
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func durationMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
