package service

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"

	"ipmanlk/breeze/internal/config"
	"ipmanlk/breeze/internal/port"
)

// mailer is the port.Mailer implementation. When SMTP is not configured
// (Host empty) it is a silent no-op so the app stays air-gapped friendly.
type mailer struct {
	cfg    config.SMTPConfig
	logger *slog.Logger
}

// NewMailer builds a port.Mailer from SMTP config. If Host is empty the
// returned mailer is disabled (Enabled() == false, Send is a no-op).
func NewMailer(cfg config.SMTPConfig, logger *slog.Logger) port.Mailer {
	if cfg.Host == "" {
		logger.Info("SMTP not configured; email delivery disabled (air-gapped mode)")
		return &mailer{cfg: cfg, logger: logger}
	}
	from := cfg.From
	if from == "" {
		from = cfg.User
	}
	logger.Info("SMTP configured", "host", cfg.Host, "port", cfg.Port, "from", from)
	return &mailer{cfg: config.SMTPConfig{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Pass:     cfg.Pass,
		From:     from,
		FromName: cfg.FromName,
		AppURL:   cfg.AppURL,
	}, logger: logger}
}

var _ port.Mailer = (*mailer)(nil)

func (m *mailer) Enabled() bool {
	return m.cfg.Host != ""
}

// Send delivers an email via SMTP. Best-effort: errors are logged and
// returned but callers should treat a non-nil error as non-fatal (the
// triggering action has already succeeded).
func (m *mailer) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	if !m.Enabled() {
		return nil
	}
	if to == "" {
		return nil
	}

	from := m.cfg.From
	fromName := m.cfg.FromName
	if fromName == "" {
		fromName = "Breeze"
	}

	headers := map[string]string{
		"From":         sanitizeHeaderValue(fmt.Sprintf("%s <%s>", fromName, from)),
		"To":           sanitizeHeaderValue(to),
		"Subject":      sanitizeHeaderValue(subject),
		"MIME-Version": "1.0",
	}

	var body strings.Builder
	if textBody != "" && htmlBody != "" {
		// Multipart alternative: text + HTML.
		boundary := "breeze-" + randomBoundary()
		headers["Content-Type"] = fmt.Sprintf("multipart/alternative; boundary=%s", boundary)
		for k, v := range headers {
			fmt.Fprintf(&body, "%s: %s\r\n", k, v)
		}
		body.WriteString("\r\n")
		fmt.Fprintf(&body, "--%s\r\n", boundary)
		body.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		body.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		body.WriteString(textBody)
		body.WriteString("\r\n\r\n")
		fmt.Fprintf(&body, "--%s\r\n", boundary)
		body.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		body.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		body.WriteString(htmlBody)
		body.WriteString("\r\n\r\n")
		fmt.Fprintf(&body, "--%s--\r\n", boundary)
	} else if htmlBody != "" {
		headers["Content-Type"] = "text/html; charset=utf-8"
		headers["Content-Transfer-Encoding"] = "8bit"
		for k, v := range headers {
			fmt.Fprintf(&body, "%s: %s\r\n", k, v)
		}
		body.WriteString("\r\n")
		body.WriteString(htmlBody)
	} else {
		headers["Content-Type"] = "text/plain; charset=utf-8"
		headers["Content-Transfer-Encoding"] = "8bit"
		for k, v := range headers {
			fmt.Fprintf(&body, "%s: %s\r\n", k, v)
		}
		body.WriteString("\r\n")
		body.WriteString(textBody)
	}

	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprintf("%d", m.cfg.Port))
	var auth smtp.Auth
	if m.cfg.User != "" {
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)
	}

	// Use STARTTLS when the server advertises it (port 587). For implicit
	// TLS (port 465) dial a TLS connection directly.
	if m.cfg.Port == 465 {
		if err := m.sendTLS(ctx, addr, from, []string{to}, body.String(), auth); err != nil {
			m.logger.Warn("send email (implicit TLS)", "error", err, "to", to, "subject", subject)
			return err
		}
		return nil
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(body.String())); err != nil {
		m.logger.Warn("send email", "error", err, "to", to, "subject", subject)
		return err
	}
	return nil
}

// sendTLS dials an implicit-TLS connection (port 465) and sends the message.
// The TLS handshake wraps the TCP connection BEFORE any SMTP bytes are
// exchanged: this is RFC 8314 §3 implicit TLS, not STARTTLS upgrading an
// already-plaintext connection.
func (m *mailer) sendTLS(ctx context.Context, addr, from string, to []string, msg string, auth smtp.Auth) error {
	tlsCfg := &tls.Config{ServerName: m.cfg.Host}
	tlsDialer := &tls.Dialer{NetDialer: &net.Dialer{}, Config: tlsCfg}
	conn, err := tlsDialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("tls dial %s: %w", addr, err)
	}
	defer conn.Close()
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	// No StartTLS call; the connection is already TLS-wrapped (implicit TLS).
	// RFC 8314 §3: the TLS handshake occurs on the raw TCP connection before
	// any SMTP protocol data is exchanged.
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt to: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close body: %w", err)
	}
	return c.Quit()
}

// randomBoundary returns a short unique string for multipart boundaries.
func randomBoundary() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b[:])
}

// sanitizeHeaderValue strips CR/LF from an SMTP header value so user-derived
// content (subject, display names, recipient addresses) cannot inject
// additional MIME headers (RFC 5322 header injection).
func sanitizeHeaderValue(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	return strings.ReplaceAll(v, "\n", "")
}
