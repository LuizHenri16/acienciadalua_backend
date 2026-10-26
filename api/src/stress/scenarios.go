package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Scenario produces the requests executed by the load test.
type Scenario interface {
	Name() string

	// Setup runs once before the workers start (e.g. authenticate).
	Setup(ctx context.Context, client *http.Client) error

	// Next returns the next request for the given worker.
	Next(ctx context.Context, state *WorkerState) (*http.Request, error)

	// Cleanup runs once after the workers stop (e.g. delete test products).
	Cleanup(ctx context.Context, client *http.Client)
}

// WorkerState is mutable state owned by a single worker goroutine, so it needs
// no synchronization.
type WorkerState struct {
	ID        int
	Seq       int64
	Step      int
	ProductID string
	Title     string
}

// AfterHook is implemented by scenarios that need to inspect a response (e.g.
// to learn the id of a created product).
type AfterHook interface {
	After(state *WorkerState, status int, body []byte)
}

func buildScenario(opts *Options) (Scenario, error) {
	switch opts.Scenario {
	case "read":
		return newReadScenario(opts), nil
	case "webhook":
		return &webhookScenario{baseURL: opts.BaseURL}, nil
	case "auth":
		return newAuthScenario(opts)
	case "crud":
		return newCRUDScenario(opts), nil
	case "mixed":
		return newMixedScenario(opts), nil
	default:
		return nil, fmt.Errorf("cenário %q desconhecido", opts.Scenario)
	}
}

// ─── read ────────────────────────────────────────────────────────────────────

// readScenario exercises only public GET endpoints. This is the safest
// default: it never writes anything.
type readScenario struct {
	baseURL string
	paths   []string
	counter atomic.Int64
}

func newReadScenario(opts *Options) *readScenario {
	paths := []string{
		"/health",
		"/products",
		"/products?category=STUDENT",
		"/products?isActive=true",
	}
	if opts.ProductID != "" {
		paths = append(paths, "/products/"+opts.ProductID)
	}

	return &readScenario{baseURL: opts.BaseURL, paths: paths}
}

func (s *readScenario) Name() string                              { return "read" }
func (s *readScenario) Setup(context.Context, *http.Client) error { return nil }
func (s *readScenario) Cleanup(context.Context, *http.Client)     {}

func (s *readScenario) Next(ctx context.Context, _ *WorkerState) (*http.Request, error) {
	index := int(s.counter.Add(1)-1) % len(s.paths)
	return newRequest(ctx, http.MethodGet, s.baseURL, s.paths[index], nil)
}

// ─── webhook ─────────────────────────────────────────────────────────────────

// webhookScenario sends notifications that the handler ignores without calling
// Mercado Pago: no payment id is resolvable, so the request only exercises the
// HTTP/JSON/validation path. It never creates purchases.
type webhookScenario struct {
	baseURL string
	counter atomic.Int64
}

var webhookBodies = [][]byte{
	[]byte(`{"type":"merchant_order","data":{"id":1}}`),
	[]byte(`{"type":"payment","data":{}}`),
	[]byte(`{"action":"payment.created","type":"payment"}`),
}

func (s *webhookScenario) Name() string                              { return "webhook" }
func (s *webhookScenario) Setup(context.Context, *http.Client) error { return nil }
func (s *webhookScenario) Cleanup(context.Context, *http.Client)     {}

func (s *webhookScenario) Next(ctx context.Context, _ *WorkerState) (*http.Request, error) {
	index := int(s.counter.Add(1)-1) % len(webhookBodies)
	return newRequest(ctx, http.MethodPost, s.baseURL, "/payment/webhook", webhookBodies[index])
}

// ─── auth ────────────────────────────────────────────────────────────────────

// authScenario signs admin and/or customer users in. Expect 429 after a few
// requests: the auth routes are rate limited per IP. Use -distribute-ip to
// measure the sign-in path itself rather than the limiter.
type authScenario struct {
	baseURL  string
	admin    []byte
	customer []byte
	counter  atomic.Int64
}

func newAuthScenario(opts *Options) (*authScenario, error) {
	if !opts.hasAdminCredentials() && !opts.hasCustomerCredentials() {
		return nil, errors.New("cenário auth requer -admin-email/-admin-password ou -customer-email/-customer-password")
	}

	s := &authScenario{baseURL: opts.BaseURL}
	if opts.hasAdminCredentials() {
		s.admin = mustJSON(map[string]string{
			"email":    opts.AdminEmail,
			"password": opts.AdminPassword,
		})
	}
	if opts.hasCustomerCredentials() {
		s.customer = mustJSON(map[string]string{
			"email":    opts.CustomerEmail,
			"password": opts.CustomerPassword,
		})
	}

	return s, nil
}

func (s *authScenario) Name() string                              { return "auth" }
func (s *authScenario) Setup(context.Context, *http.Client) error { return nil }
func (s *authScenario) Cleanup(context.Context, *http.Client)     {}

func (s *authScenario) Next(ctx context.Context, _ *WorkerState) (*http.Request, error) {
	seq := s.counter.Add(1)

	if s.admin != nil && (s.customer == nil || seq%2 == 1) {
		return newRequest(ctx, http.MethodPost, s.baseURL, "/auth/admin/signin", s.admin)
	}

	return newRequest(ctx, http.MethodPost, s.baseURL, "/auth/user/signin", s.customer)
}

// ─── crud ────────────────────────────────────────────────────────────────────

const (
	crudStepCreate = iota
	crudStepUpdate
	crudStepToggle
	crudStepGet
	crudStepDelete
)

// crudScenario runs full product lifecycles: create → update → toggle → get →
// delete. Products are created with a unique title and every id is tracked so
// Cleanup can remove leftovers after an interrupted run.
type crudScenario struct {
	baseURL  string
	token    string
	email    string
	password string

	mu      sync.Mutex
	created map[string]struct{}
}

func newCRUDScenario(opts *Options) *crudScenario {
	return &crudScenario{
		baseURL:  opts.BaseURL,
		token:    opts.AdminToken,
		email:    opts.AdminEmail,
		password: opts.AdminPassword,
		created:  make(map[string]struct{}),
	}
}

func (s *crudScenario) Name() string { return "crud" }

func (s *crudScenario) Setup(ctx context.Context, client *http.Client) error {
	if s.token != "" {
		return nil
	}

	token, err := adminSignIn(ctx, client, s.baseURL, s.email, s.password)
	if err != nil {
		return err
	}

	s.token = token
	return nil
}

func (s *crudScenario) Next(ctx context.Context, worker *WorkerState) (*http.Request, error) {
	if worker.ProductID == "" && worker.Step != crudStepCreate {
		worker.Step = crudStepCreate
	}

	switch worker.Step {
	case crudStepCreate:
		worker.Seq++
		worker.Title = fmt.Sprintf(
			"stress-%d-%d-%d",
			time.Now().UnixNano(),
			worker.ID,
			worker.Seq,
		)
		payload := mustJSON(map[string]any{
			"title":       worker.Title,
			"description": "produto criado pelo teste de estresse",
			"price":       1.5,
			"isActive":    false,
			"category":    "STUDENT",
		})
		return s.withAuth(newRequest(ctx, http.MethodPost, s.baseURL, "/products", payload))

	case crudStepUpdate:
		payload := mustJSON(map[string]any{
			"title": worker.Title + " (atualizado)",
			"price": 2.5,
		})
		return s.withAuth(newRequest(ctx, http.MethodPut, s.baseURL, "/products/"+worker.ProductID, payload))

	case crudStepToggle:
		return s.withAuth(newRequest(ctx, http.MethodPatch, s.baseURL, "/products/"+worker.ProductID+"/toggle", nil))

	case crudStepGet:
		return newRequest(ctx, http.MethodGet, s.baseURL, "/products/"+worker.ProductID, nil)

	case crudStepDelete:
		return s.withAuth(newRequest(ctx, http.MethodDelete, s.baseURL, "/products/"+worker.ProductID, nil))
	}

	worker.Step = crudStepCreate
	return s.Next(ctx, worker)
}

func (s *crudScenario) withAuth(req *http.Request, err error) (*http.Request, error) {
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	return req, nil
}

// After advances the lifecycle based on the response. On failure the worker
// restarts the cycle, and the id (if any) stays tracked for Cleanup.
func (s *crudScenario) After(worker *WorkerState, status int, body []byte) {
	ok := status >= http.StatusOK && status < http.StatusMultipleChoices

	switch worker.Step {
	case crudStepCreate:
		if ok {
			var created struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(body, &created) == nil && created.ID != "" {
				worker.ProductID = created.ID
				s.track(created.ID)
				worker.Step = crudStepUpdate
				return
			}
		}
		worker.Step = crudStepCreate

	case crudStepUpdate, crudStepToggle:
		if ok {
			worker.Step++
		} else {
			worker.Step = crudStepCreate
		}

	case crudStepGet:
		if ok {
			worker.Step = crudStepDelete
		} else {
			worker.Step = crudStepCreate
		}

	case crudStepDelete:
		if ok && worker.ProductID != "" {
			s.untrack(worker.ProductID)
		}
		worker.ProductID = ""
		worker.Step = crudStepCreate
	}
}

func (s *crudScenario) Cleanup(ctx context.Context, client *http.Client) {
	ids := s.trackedIDs()
	if len(ids) == 0 {
		return
	}

	log.Printf("limpando %d produto(s) restante(s) do teste de estresse...", len(ids))

	for _, id := range ids {
		req, err := newRequest(ctx, http.MethodDelete, s.baseURL, "/products/"+id, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+s.token)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("falha ao remover produto %s: %v", id, err)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			s.untrack(id)
		}
	}
}

func (s *crudScenario) track(id string) {
	s.mu.Lock()
	s.created[id] = struct{}{}
	s.mu.Unlock()
}

func (s *crudScenario) untrack(id string) {
	s.mu.Lock()
	delete(s.created, id)
	s.mu.Unlock()
}

func (s *crudScenario) trackedIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.created))
	for id := range s.created {
		ids = append(ids, id)
	}
	return ids
}

// ─── mixed ───────────────────────────────────────────────────────────────────

// mixedScenario is 90% public reads and 10% harmless webhook notifications.
// Auth is intentionally excluded: its rate limit would dominate the results.
type mixedScenario struct {
	read    *readScenario
	webhook *webhookScenario
	counter atomic.Int64
}

func newMixedScenario(opts *Options) *mixedScenario {
	return &mixedScenario{
		read:    newReadScenario(opts),
		webhook: &webhookScenario{baseURL: opts.BaseURL},
	}
}

func (s *mixedScenario) Name() string                              { return "mixed" }
func (s *mixedScenario) Setup(context.Context, *http.Client) error { return nil }
func (s *mixedScenario) Cleanup(context.Context, *http.Client)     {}

func (s *mixedScenario) Next(ctx context.Context, worker *WorkerState) (*http.Request, error) {
	if s.counter.Add(1)%10 == 0 {
		return s.webhook.Next(ctx, worker)
	}
	return s.read.Next(ctx, worker)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func adminSignIn(ctx context.Context, client *http.Client, baseURL, email, password string) (string, error) {
	if email == "" || password == "" {
		return "", errors.New("credenciais do admin não informadas")
	}

	body := mustJSON(map[string]string{"email": email, "password": password})
	req, err := newRequest(ctx, http.MethodPost, baseURL, "/auth/admin/signin", body)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("signin do admin falhou (status %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("resposta de signin inválida: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("resposta de signin sem access_token")
	}

	return parsed.AccessToken, nil
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}
