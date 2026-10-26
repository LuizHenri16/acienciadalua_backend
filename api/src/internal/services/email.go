// Package services holds the business rules.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/acienciadalua/api/internal/config"
)

const resendBaseURL = "https://api.resend.com"

// EmailService sends transactional e-mails through Resend.
type EmailService struct {
	cfg     *config.Config
	client  *http.Client
	baseURL string
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		cfg:     cfg,
		client:  &http.Client{Timeout: 20 * time.Second},
		baseURL: resendBaseURL,
	}
}

type resendEmail struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	ReplyTo string   `json:"reply_to,omitempty"`
}

type resendResponse struct {
	ID string `json:"id"`
}

type resendError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Name       string `json:"name"`
}

var welcomeTemplate = template.Must(template.New("welcome").Parse(`
                <h1>Olá, {{.Name}}!</h1>
                <p>Seu pagamento foi confirmado!</p>
                <p>Para acessar seus materiais, clique no link abaixo e crie sua senha de acesso:</p>
                <a href="{{.FrontendURL}}/minha-conta/signin" style="display:inline-block;padding:12px 24px;background-color:#0b7a75;color:#ffffff;text-decoration:none;border-radius:6px;font-weight:bold;">Criar minha senha</a>
                <p style="margin-top:24px;color:#64748b;font-size:13px;">Se você não realizou esta compra, ignore este email.</p>
            `))

var resetTemplate = template.Must(template.New("reset").Parse(`
                <h1>Redefinição de senha</h1>
                <p>Clique no link abaixo para redefinir sua senha:</p>
                <a href="{{.URL}}">Redefinir senha</a>
                <p>Este link expira em 1 hora.</p>
                <p>Se você não solicitou esta alteração, ignore este email.</p>
            `))

var contactTemplate = template.Must(template.New("contact").Parse(`
                <div style="font-family: sans-serif; max-width: 600px; margin: 0 auto;">
                    <h2 style="color: #0b7a75;">Nova mensagem de contato</h2>
                    <table style="width: 100%; border-collapse: collapse;">
                        <tr>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; font-weight: bold; color: #1e293b;">Nome</td>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; color: #334155;">{{.Name}}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; font-weight: bold; color: #1e293b;">Email</td>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; color: #334155;">{{.Email}}</td>
                        </tr>
                        {{if .Subject}}
                        <tr>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; font-weight: bold; color: #1e293b;">Assunto</td>
                            <td style="padding: 8px; border-bottom: 1px solid #e2e8f0; color: #334155;">{{.Subject}}</td>
                        </tr>
                        {{end}}
                    </table>
                    <div style="margin-top: 16px; padding: 16px; background: #f8fafc; border-radius: 8px;">
                        <h3 style="color: #1e293b; margin: 0 0 8px;">Mensagem</h3>
                        <p style="color: #334155; line-height: 1.6; white-space: pre-wrap;">{{.Message}}</p>
                    </div>
                    <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 24px 0;" />
                    <p style="color: #94a3b8; font-size: 12px;">Enviado do formulário de contato do site A Ciência da Lua.</p>
                </div>
            `))

// SendWelcomeEmail notifies a customer after an approved payment.
func (s *EmailService) SendWelcomeEmail(ctx context.Context, email, name string) error {
	var html bytes.Buffer
	if err := welcomeTemplate.Execute(&html, map[string]string{
		"Name":        name,
		"FrontendURL": s.cfg.FrontendURL,
	}); err != nil {
		return fmt.Errorf("rendering welcome email: %w", err)
	}

	_, err := s.send(ctx, resendEmail{
		From:    s.cfg.EmailSenderAddress,
		To:      []string{email},
		Subject: "Bem-vindo ao A Ciência da Lua",
		HTML:    html.String(),
	})
	if err != nil {
		slog.Error("[EmailService] Resend error (welcome)", "error", err)
		return fmt.Errorf("Failed to send welcome email: %w", err)
	}

	return nil
}

// SendResetPasswordEmail sends the password reset link.
func (s *EmailService) SendResetPasswordEmail(ctx context.Context, email, token string) error {
	url := fmt.Sprintf("%s/minha-conta/redefinir-senha?token=%s", s.cfg.FrontendURL, token)

	var html bytes.Buffer
	if err := resetTemplate.Execute(&html, map[string]string{"URL": url}); err != nil {
		return fmt.Errorf("rendering reset email: %w", err)
	}

	id, err := s.send(ctx, resendEmail{
		From:    s.cfg.EmailSenderAddress,
		To:      []string{email},
		Subject: "Redefinição de senha - A Ciência da Lua",
		HTML:    html.String(),
	})
	if err != nil {
		slog.Error("[EmailService] Resend error (reset)", "error", err)
		return fmt.Errorf("Failed to send reset email: %w", err)
	}

	slog.Info("[EmailService] Reset email sent", "email_id", id)
	return nil
}

// SendContactEmail forwards a website contact form message.
func (s *EmailService) SendContactEmail(ctx context.Context, name, email, subject, message string) error {
	var html bytes.Buffer
	if err := contactTemplate.Execute(&html, map[string]string{
		"Name":    name,
		"Email":   email,
		"Subject": subject,
		"Message": message,
	}); err != nil {
		return fmt.Errorf("rendering contact email: %w", err)
	}

	emailSubject := fmt.Sprintf("[Contato Site] Mensagem de %s", name)
	if subject != "" {
		emailSubject = "[Contato Site] " + subject
	}

	id, err := s.send(ctx, resendEmail{
		From:    s.cfg.EmailSenderAddress,
		To:      []string{s.cfg.ContactEmail},
		Subject: emailSubject,
		HTML:    html.String(),
		ReplyTo: email,
	})
	if err != nil {
		slog.Error("[EmailService] Resend error (contact)", "error", err)
		return fmt.Errorf("Failed to send contact email: %w", err)
	}

	slog.Info("[EmailService] Contact email sent", "email_id", id)
	return nil
}

func (s *EmailService) send(ctx context.Context, payload resendEmail) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+s.cfg.ResendAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var apiErr resendError
		if err := json.Unmarshal(responseBody, &apiErr); err == nil && apiErr.Message != "" {
			return "", fmt.Errorf("%s", apiErr.Message)
		}
		return "", fmt.Errorf("resend request failed with status %d", response.StatusCode)
	}

	var parsed resendResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", nil // response without a body/id is still a success
	}

	return parsed.ID, nil
}

func isEmail(value string) bool {
	// Basic address validation: no whitespace, one "@" with non-empty local
	// part and domain, and a dot in the domain.
	if strings.ContainsAny(value, " \t\n\r") {
		return false
	}
	at := strings.LastIndex(value, "@")
	if at <= 0 || at == len(value)-1 {
		return false
	}
	domain := value[at+1:]
	dot := strings.Index(domain, ".")
	return dot > 0 && dot < len(domain)-1
}
