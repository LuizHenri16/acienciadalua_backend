// Command stress runs load tests against a running instance of the API
// (default http://localhost:3001).
//
// It is intentionally dependency-free (standard library only) so it can be
// executed with `go run` against local, staging or production-like
// environments.
//
// Usage examples:
//
//	go run ./stress -scenario=read -concurrency=20 -requests=5000
//	go run ./stress -scenario=read -duration=30s -concurrency=50
//	go run ./stress -scenario=webhook -requests=2000
//	go run ./stress -scenario=auth -admin-email=... -admin-password=... -distribute-ip
//	go run ./stress -scenario=crud -admin-email=... -admin-password=... -requests=200
//
// See stress/README.md for the full reference.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// All the work happens in run() so deferred calls (notably the scenario
	// cleanup) also execute on error and threshold-exceeded exits.
	os.Exit(run())
}

func run() int {
	opts := parseFlags()

	if err := opts.validate(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 2
	}

	scenario, err := buildScenario(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := newHTTPClient(opts)

	if err := scenario.Setup(ctx, client); err != nil {
		fmt.Fprintln(os.Stderr, "erro no setup do cenário:", err)
		return 1
	}

	// Cleanup always runs (even on Ctrl+C or a threshold failure) so the crud
	// scenario never leaves products behind. It uses a fresh context because
	// the run context is already cancelled at that point.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		scenario.Cleanup(cleanupCtx, client)
	}()

	report, err := Run(ctx, opts, client, scenario)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}

	if opts.JSON {
		if err := printJSON(os.Stdout, report); err != nil {
			fmt.Fprintln(os.Stderr, "erro ao gerar JSON:", err)
			return 1
		}
	} else {
		printReport(os.Stdout, report)
	}

	if limit := opts.MaxErrorRate; limit > 0 && report.ErrorRate() > limit {
		fmt.Fprintf(
			os.Stderr,
			"\ntaxa de erro %.2f%% acima do limite de %.2f%%\n",
			report.ErrorRate()*100,
			limit*100,
		)
		return 1
	}

	return 0
}
