package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	latencies := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	}
	sorted := append([]time.Duration(nil), latencies...)

	tests := []struct {
		name string
		p    float64
		want time.Duration
	}{
		{name: "zero returns min", p: 0, want: 1 * time.Millisecond},
		{name: "p50", p: 0.50, want: 3 * time.Millisecond},
		{name: "p90", p: 0.90, want: 5 * time.Millisecond},
		{name: "p99", p: 0.99, want: 5 * time.Millisecond},
		{name: "one returns max", p: 1, want: 5 * time.Millisecond},
		{name: "negative clamps", p: -0.5, want: 1 * time.Millisecond},
		{name: "above one clamps", p: 2, want: 5 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentile(sorted, tt.p); got != tt.want {
				t.Fatalf("percentile(%v) = %v, quer %v", tt.p, got, tt.want)
			}
		})
	}

	if got := percentile(nil, 0.5); got != 0 {
		t.Fatalf("percentile de slice vazio = %v, quer 0", got)
	}
}

func TestReportRates(t *testing.T) {
	report := &Report{
		Elapsed:  2 * time.Second,
		Requests: 10,
		StatusCounts: map[int]int64{
			200: 5,
			404: 2,
			500: 2,
		},
		TransportErrors: 1,
	}

	if got, want := report.Throughput(), 5.0; got != want {
		t.Fatalf("Throughput() = %v, quer %v", got, want)
	}

	if got, want := report.SuccessRate(), 0.5; got != want {
		t.Fatalf("SuccessRate() = %v, quer %v", got, want)
	}

	// 1 erro de transporte + 2 respostas 5xx em 10 requisições.
	if got, want := report.ErrorRate(), 0.3; got != want {
		t.Fatalf("ErrorRate() = %v, quer %v", got, want)
	}

	if got := report.serverErrors(); got != 2 {
		t.Fatalf("serverErrors() = %d, quer 2", got)
	}

	empty := &Report{}
	if empty.Throughput() != 0 || empty.SuccessRate() != 0 || empty.ErrorRate() != 0 {
		t.Fatal("relatório vazio deve ter taxas zeradas")
	}
}

func TestOptionsValidate(t *testing.T) {
	base := func() *Options {
		return &Options{
			BaseURL:     "http://localhost:3001/",
			Scenario:    "read",
			Concurrency: 2,
			Requests:    10,
			Timeout:     time.Second,
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Options)
		wantErr string
	}{
		{name: "válido", mutate: func(*Options) {}},
		{
			name:    "esquema inválido",
			mutate:  func(o *Options) { o.BaseURL = "ftp://localhost" },
			wantErr: "http ou https",
		},
		{
			name:    "sem host",
			mutate:  func(o *Options) { o.BaseURL = "http://" },
			wantErr: "sem host",
		},
		{
			name:    "concorrência zero",
			mutate:  func(o *Options) { o.Concurrency = 0 },
			wantErr: "concurrency",
		},
		{
			name:    "sem limite",
			mutate:  func(o *Options) { o.Requests = 0 },
			wantErr: "requests > 0",
		},
		{
			name:    "rate negativo",
			mutate:  func(o *Options) { o.Rate = -1 },
			wantErr: "rate",
		},
		{
			name:    "timeout zero",
			mutate:  func(o *Options) { o.Timeout = 0 },
			wantErr: "timeout",
		},
		{
			name:    "taxa de erro fora do intervalo",
			mutate:  func(o *Options) { o.MaxErrorRate = 1.5 },
			wantErr: "max-error-rate",
		},
		{
			name:    "cenário desconhecido",
			mutate:  func(o *Options) { o.Scenario = "bogus" },
			wantErr: "cenário",
		},
		{
			name:    "auth sem credenciais",
			mutate:  func(o *Options) { o.Scenario = "auth" },
			wantErr: "admin-email",
		},
		{
			name:    "crud sem token nem credenciais",
			mutate:  func(o *Options) { o.Scenario = "crud" },
			wantErr: "admin-token",
		},
		{
			name: "crud remoto protegido",
			mutate: func(o *Options) {
				o.Scenario = "crud"
				o.AdminToken = "token"
				o.BaseURL = "https://example.com"
			},
			wantErr: "allow-remote-writes",
		},
		{
			name: "crud local permitido",
			mutate: func(o *Options) {
				o.Scenario = "crud"
				o.AdminToken = "token"
				o.BaseURL = "http://127.0.0.1:3001"
			},
		},
		{
			name: "crud remoto com confirmação",
			mutate: func(o *Options) {
				o.Scenario = "crud"
				o.AdminToken = "token"
				o.BaseURL = "https://example.com"
				o.AllowRemoteWrites = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := base()
			tt.mutate(opts)

			err := opts.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() retornou erro inesperado: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validate() deveria falhar com %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("erro %q não contém %q", err, tt.wantErr)
			}
		})
	}

	opts := base()
	if err := opts.validate(); err != nil {
		t.Fatalf("validate() erro inesperado: %v", err)
	}
	if opts.BaseURL != "http://localhost:3001" {
		t.Fatalf("BaseURL = %q, quer sem barra final", opts.BaseURL)
	}
}

func TestIsLocalURL(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{raw: "http://localhost:3001", want: true},
		{raw: "http://127.0.0.1:3001", want: true},
		{raw: "http://[::1]:3001", want: true},
		{raw: "http://0.0.0.0:3001", want: true},
		{raw: "http://192.168.0.10:3001", want: false},
		{raw: "https://api.example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			parsed, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", tt.raw, err)
			}
			if got := isLocalURL(parsed); got != tt.want {
				t.Fatalf("isLocalURL(%q) = %v, quer %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestAdminSignIn(t *testing.T) {
	var seen atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Add(1)

		if r.Method != http.MethodPost || r.URL.Path != "/auth/admin/signin" {
			http.NotFound(w, r)
			return
		}

		var payload struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"message":"json inválido"}`, http.StatusBadRequest)
			return
		}

		if payload.Email != "admin@example.com" || payload.Password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Credenciais inválidas","statusCode":401}`))
			return
		}

		_, _ = w.Write([]byte(`{"access_token":"jwt-token"}`))
	}))
	defer server.Close()

	token, err := adminSignIn(context.Background(), server.Client(), server.URL, "admin@example.com", "secret")
	if err != nil {
		t.Fatalf("adminSignIn() erro: %v", err)
	}
	if token != "jwt-token" {
		t.Fatalf("token = %q, quer %q", token, "jwt-token")
	}

	_, err = adminSignIn(context.Background(), server.Client(), server.URL, "admin@example.com", "errada")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("esperava erro 401, obtido %v", err)
	}

	if _, err := adminSignIn(context.Background(), server.Client(), server.URL, "", ""); err == nil {
		t.Fatal("esperava erro sem credenciais")
	}

	if seen.Load() != 2 {
		t.Fatalf("servidor recebeu %d requisições, quer 2", seen.Load())
	}
}

func TestRunReadScenario(t *testing.T) {
	var hits atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	opts := &Options{
		BaseURL:     server.URL,
		Scenario:    "read",
		Concurrency: 3,
		Requests:    25,
		Timeout:     5 * time.Second,
	}

	report, err := Run(context.Background(), opts, newHTTPClient(opts), newReadScenario(opts))
	if err != nil {
		t.Fatalf("Run() erro: %v", err)
	}

	if report.Requests != 25 {
		t.Fatalf("Requests = %d, quer 25", report.Requests)
	}
	if got := hits.Load(); got != 25 {
		t.Fatalf("servidor recebeu %d requisições, quer 25", got)
	}
	if report.StatusCounts[http.StatusOK] != 25 {
		t.Fatalf("status 200 = %d, quer 25 (todos: %v)", report.StatusCounts[http.StatusOK], report.StatusCounts)
	}
	if report.TransportErrors != 0 || report.BuildErrors != 0 {
		t.Fatalf("erros inesperados: transporte=%d build=%d", report.TransportErrors, report.BuildErrors)
	}
	if report.SuccessRate() != 1 || report.ErrorRate() != 0 {
		t.Fatalf("taxas inesperadas: sucesso=%v erro=%v", report.SuccessRate(), report.ErrorRate())
	}
	if report.Elapsed <= 0 || report.Throughput() <= 0 {
		t.Fatalf("métricas de tempo inválidas: elapsed=%v throughput=%v", report.Elapsed, report.Throughput())
	}
	if report.Bytes <= 0 {
		t.Fatalf("Bytes = %d, quer > 0", report.Bytes)
	}
	if report.Latency.Max <= 0 || report.Latency.P50 <= 0 {
		t.Fatalf("latências inválidas: %+v", report.Latency)
	}
}

// TestCRUDScenarioLifecycle exercises the full create → update → toggle → get →
// delete cycle and asserts that cleanup removes leftovers.
func TestCRUDScenarioLifecycle(t *testing.T) {
	const token = "test-token"

	var (
		mu          sync.Mutex
		created     = map[string]bool{}
		deleted     = map[string]bool{}
		authFailure atomic.Int64
		idSeq       atomic.Int64
	)

	requireAuth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "Bearer "+token {
			authFailure.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Unauthorized","statusCode":401}`))
			return false
		}
		return true
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/products":
			if !requireAuth(w, r) {
				return
			}

			var payload struct {
				Title    string  `json:"title"`
				Price    float64 `json:"price"`
				Category string  `json:"category"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, `{"message":"json inválido"}`, http.StatusBadRequest)
				return
			}
			if !strings.HasPrefix(payload.Title, "stress-") || payload.Price <= 0 || payload.Category != "STUDENT" {
				http.Error(w, `{"message":"payload inesperado"}`, http.StatusBadRequest)
				return
			}

			id := fmt.Sprintf("product-%d", idSeq.Add(1))
			mu.Lock()
			created[id] = true
			mu.Unlock()

			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"id":%q}`, id)

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/products/"):
			if !requireAuth(w, r) {
				return
			}
			id := strings.TrimPrefix(r.URL.Path, "/products/")
			mu.Lock()
			deleted[id] = true
			mu.Unlock()
			_, _ = w.Write([]byte(`{"ok":true}`))

		case strings.HasPrefix(r.URL.Path, "/products/"):
			id := strings.TrimPrefix(r.URL.Path, "/products/")
			if strings.HasSuffix(id, "/toggle") {
				id = strings.TrimSuffix(id, "/toggle")
				if !requireAuth(w, r) {
					return
				}
			}
			_, _ = fmt.Fprintf(w, `{"id":%q}`, id)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	opts := &Options{
		BaseURL:     server.URL,
		Scenario:    "crud",
		Concurrency: 1,
		Requests:    100,
		Timeout:     5 * time.Second,
		AdminToken:  token,
	}
	scenario := newCRUDScenario(opts)
	client := newHTTPClient(opts)

	if err := scenario.Setup(context.Background(), client); err != nil {
		t.Fatalf("Setup() erro: %v", err)
	}

	worker := &WorkerState{ID: 1}

	// Dois ciclos completos de 5 requisições.
	for cycle := 0; cycle < 2; cycle++ {
		for step := 0; step < 5; step++ {
			req, err := scenario.Next(context.Background(), worker)
			if err != nil {
				t.Fatalf("Next() erro no ciclo %d passo %d: %v", cycle, step, err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("requisição falhou: %v", err)
			}
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				t.Fatalf("leitura do corpo falhou: %v", err)
			}

			scenario.After(worker, resp.StatusCode, body)
		}
	}

	if worker.Seq != 2 {
		t.Fatalf("Seq = %d, quer 2 ciclos concluídos", worker.Seq)
	}
	if worker.ProductID != "" {
		t.Fatalf("ProductID = %q, quer vazio após o delete", worker.ProductID)
	}
	if ids := scenario.trackedIDs(); len(ids) != 0 {
		t.Fatalf("produtos ainda rastreados: %v", ids)
	}
	if got := authFailure.Load(); got != 0 {
		t.Fatalf("requisições sem auth: %d", got)
	}

	mu.Lock()
	createdCount, deletedCount := len(created), len(deleted)
	mu.Unlock()

	if createdCount != 2 || deletedCount != 2 {
		t.Fatalf("criados=%d removidos=%d, quer 2/2", createdCount, deletedCount)
	}

	// Simula uma execução interrompida: um produto fica órfão e o Cleanup deve
	// removê-lo.
	scenario.track("orphan-product")
	scenario.Cleanup(context.Background(), client)

	mu.Lock()
	_, orphanDeleted := deleted["orphan-product"]
	mu.Unlock()

	if !orphanDeleted {
		t.Fatal("Cleanup não removeu o produto órfão")
	}
	if ids := scenario.trackedIDs(); len(ids) != 0 {
		t.Fatalf("produtos ainda rastreados após Cleanup: %v", ids)
	}
}
