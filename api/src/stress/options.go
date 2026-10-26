package main

import (
	"flag"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Options holds every flag accepted by the stress command.
type Options struct {
	BaseURL     string
	Scenario    string
	Concurrency int

	// Requests and Duration bound the run. When both are set the test stops at
	// whichever limit is reached first. At least one must be set.
	Requests int64
	Duration time.Duration

	// Rate caps the global number of requests per second (0 = unlimited).
	Rate float64

	Timeout   time.Duration
	ProductID string

	AdminEmail    string
	AdminPassword string
	AdminToken    string

	CustomerEmail    string
	CustomerPassword string

	// DistributeIP sends a unique X-Forwarded-For per request, which bypasses
	// the per-IP rate limit. Only use it on environments you control.
	DistributeIP bool

	JSON         bool
	MaxErrorRate float64

	// AllowRemoteWrites permits destructive scenarios against non-loopback
	// hosts.
	AllowRemoteWrites bool
}

func parseFlags() *Options {
	opts := &Options{}

	flag.StringVar(&opts.BaseURL, "base-url", "http://localhost:3000", "URL base da API")
	flag.StringVar(&opts.Scenario, "scenario", "read", "cenário: read, webhook, auth, crud ou mixed")
	flag.IntVar(&opts.Concurrency, "concurrency", 10, "número de workers concorrentes")
	flag.Int64Var(&opts.Requests, "requests", 1000, "total de requisições (0 = usar apenas -duration)")
	flag.DurationVar(&opts.Duration, "duration", 0, "duração do teste (ex.: 30s); para no primeiro limite atingido")
	flag.Float64Var(&opts.Rate, "rate", 0, "limite global de requisições por segundo (0 = sem limite)")
	flag.DurationVar(&opts.Timeout, "timeout", 10*time.Second, "timeout por requisição")
	flag.StringVar(&opts.ProductID, "product-id", "", "ID de produto incluído no cenário read")
	flag.StringVar(&opts.AdminEmail, "admin-email", "", "e-mail do admin (cenários auth e crud)")
	flag.StringVar(&opts.AdminPassword, "admin-password", "", "senha do admin (cenários auth e crud)")
	flag.StringVar(&opts.AdminToken, "admin-token", "", "access token do admin (evita o signin no setup do crud)")
	flag.StringVar(&opts.CustomerEmail, "customer-email", "", "e-mail do cliente (cenário auth)")
	flag.StringVar(&opts.CustomerPassword, "customer-password", "", "senha do cliente (cenário auth)")
	flag.BoolVar(&opts.DistributeIP, "distribute-ip", false, "envia X-Forwarded-For único por requisição (contorna o rate limit por IP)")
	flag.BoolVar(&opts.JSON, "json", false, "imprime o relatório em JSON")
	flag.Float64Var(&opts.MaxErrorRate, "max-error-rate", 0, "falha (exit 1) se a taxa de erro (transporte + 5xx) exceder este valor, ex.: 0.01")
	flag.BoolVar(&opts.AllowRemoteWrites, "allow-remote-writes", false, "permite cenários destrutivos contra hosts remotos")

	flag.Parse()

	// `-duration` alone should not be capped by the default -requests value.
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if explicit["duration"] && !explicit["requests"] {
		opts.Requests = 0
	}

	return opts
}

func (o *Options) validate() error {
	o.BaseURL = strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")

	parsed, err := url.Parse(o.BaseURL)
	if err != nil {
		return fmt.Errorf("base-url inválida: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base-url deve usar http ou https (recebido %q)", o.BaseURL)
	}
	if parsed.Host == "" {
		return fmt.Errorf("base-url sem host: %q", o.BaseURL)
	}

	if o.Concurrency < 1 {
		return fmt.Errorf("concurrency deve ser >= 1")
	}
	if o.Requests < 0 {
		return fmt.Errorf("requests não pode ser negativo")
	}
	if o.Duration < 0 {
		return fmt.Errorf("duration não pode ser negativa")
	}
	if o.Requests == 0 && o.Duration == 0 {
		return fmt.Errorf("informe -requests > 0 ou -duration > 0")
	}
	if o.Rate < 0 {
		return fmt.Errorf("rate não pode ser negativo")
	}
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout deve ser > 0")
	}
	if o.MaxErrorRate < 0 || o.MaxErrorRate > 1 {
		return fmt.Errorf("max-error-rate deve estar entre 0 e 1")
	}

	switch o.Scenario {
	case "read", "webhook", "auth", "crud", "mixed":
	default:
		return fmt.Errorf("cenário %q inválido (use read, webhook, auth, crud ou mixed)", o.Scenario)
	}

	if o.Scenario == "auth" && !o.hasAdminCredentials() && !o.hasCustomerCredentials() {
		return fmt.Errorf("cenário auth requer -admin-email/-admin-password ou -customer-email/-customer-password")
	}

	if o.Scenario == "crud" {
		if o.AdminToken == "" && !o.hasAdminCredentials() {
			return fmt.Errorf("cenário crud requer -admin-token ou -admin-email/-admin-password")
		}
		if !o.AllowRemoteWrites && !isLocalURL(parsed) {
			return fmt.Errorf("cenário crud cria e apaga produtos; use -allow-remote-writes para confirmar que %q é descartável", o.BaseURL)
		}
	}

	return nil
}

func (o *Options) hasAdminCredentials() bool {
	return o.AdminEmail != "" && o.AdminPassword != ""
}

func (o *Options) hasCustomerCredentials() bool {
	return o.CustomerEmail != "" && o.CustomerPassword != ""
}

func isLocalURL(parsed *url.URL) bool {
	host := parsed.Hostname()
	if host == "localhost" || host == "0.0.0.0" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
