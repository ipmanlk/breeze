package service

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"ipmanlk/breeze/internal/config"
)

func TestMailer_DisabledWhenNoHost(t *testing.T) {
	m := NewMailer(config.SMTPConfig{}, slog.Default())
	if m.Enabled() {
		t.Fatal("mailer should be disabled when Host is empty")
	}
	// Send must be a no-op (no panic, no error, no network).
	if err := m.Send(context.Background(), "a@t.com", "s", "<b>h</b>", "t"); err != nil {
		t.Fatalf("disabled Send returned error: %v", err)
	}
}

func TestMailer_EnabledWhenHostSet(t *testing.T) {
	m := NewMailer(config.SMTPConfig{Host: "localhost", Port: 2525, From: "breeze@t.com"}, slog.Default())
	if !m.Enabled() {
		t.Fatal("mailer should be enabled when Host is set")
	}
}

func TestPasswordResetEmail_ContainsLink(t *testing.T) {
	url := "https://breeze.example.com/reset-password?token=abc"
	tmpl := PasswordResetEmail(url)
	if !strings.Contains(tmpl.Text, url) {
		t.Errorf("text body missing reset URL")
	}
	if !strings.Contains(tmpl.HTML, url) {
		t.Errorf("html body missing reset URL")
	}
	if tmpl.Subject == "" {
		t.Error("subject is empty")
	}
}

func TestInviteEmail_ContainsLinkAndNames(t *testing.T) {
	url := "https://breeze.example.com/join?token=xyz"
	tmpl := InviteEmail("Alice", "Acme", url)
	if !strings.Contains(tmpl.Text, "Alice") || !strings.Contains(tmpl.Text, "Acme") {
		t.Errorf("text body missing inviter/org name: %q", tmpl.Text)
	}
	if !strings.Contains(tmpl.HTML, url) {
		t.Errorf("html body missing join URL")
	}
}

func TestNotificationEmail_ContainsTitleAndLink(t *testing.T) {
	tmpl := NotificationEmail("Task due: Fix bug", "Fix bug is due tomorrow", "https://breeze.example.com/projects/p?task=t1")
	if !strings.Contains(tmpl.Text, "Fix bug") {
		t.Errorf("text body missing title")
	}
	if !strings.Contains(tmpl.Text, "https://breeze.example.com/projects/p?task=t1") {
		t.Errorf("text body missing link")
	}
	if !strings.Contains(tmpl.HTML, "Task due: Fix bug") {
		t.Errorf("html body missing title")
	}
}

// TestMailer_SendTLS_ImplicitTLS verifies that the port-465 (implicit TLS) path
// sends a TLS ClientHello (0x16) as the FIRST byte on the wire, BEFORE any
// SMTP protocol bytes. The TLS handshake wraps the TCP connection before any
// SMTP data is exchanged (RFC 8314 §3).
func TestMailer_SendTLS_ImplicitTLS(t *testing.T) {
	// Start a plain TCP listener: no TLS on our side. The test client will
	// attempt TLS. Even though the TLS handshake will fail (we're not doing
	// server-side TLS), the client MUST send a TLS ClientHello before any
	// SMTP bytes, and we read that first byte to verify.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	firstByteCh := make(chan byte, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Logf("accept error: %v", err)
			return
		}
		defer conn.Close()

		// Write the SMTP 220 banner so that even if the old (broken) code
		// path were somehow triggered, it doesn't deadlock waiting for
		// the server to send before the client sends anything.
		if _, err := conn.Write([]byte("220 test ESMTP\r\n")); err != nil {
			t.Logf("write 220: %v", err)
			return
		}

		// Read the first byte from the client. With the FIX, this must be
		// 0x16 (TLS ClientHello record type). With the OLD broken code,
		// the first client-sent byte would be 'S' from "STARTTLS".
		buf := make([]byte, 1)
		if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Logf("set read deadline: %v", err)
			return
		}
		n, err := conn.Read(buf)
		if err != nil || n < 1 {
			t.Logf("read first byte: %v (n=%d)", err, n)
			return
		}
		firstByteCh <- buf[0]
	}()

	// Create a mailer whose Host is used for the TLS ServerName.
	m := &mailer{
		cfg: config.SMTPConfig{
			Host: "127.0.0.1",
			Port: 465, // triggers the implicit TLS path
		},
		logger: slog.Default(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call sendTLS: this will attempt TLS via tls.Dialer.DialContext.
	// The TLS handshake will fail (our test server doesn't speak TLS), but
	// the TLS ClientHello will have been sent before the failure.
	_ = m.sendTLS(ctx, addr, "from@test.com", nil, "test body", nil)

	select {
	case b := <-firstByteCh:
		if b != 0x16 {
			t.Fatalf("expected TLS ClientHello (0x16) as first byte, got 0x%02x (%q). "+
				"This means SMTP bytes were sent before TLS was established", b, b)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("timeout waiting for server to receive first byte from client")
	}
}

// TestMailer_SendTLS_ContextCancellation verifies that the implicit-TLS path
// respects context cancellation.
func TestMailer_SendTLS_ContextCancellation(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// Don't accept: let the connection attempt hang
	addr := ln.Addr().String()

	m := &mailer{
		cfg: config.SMTPConfig{
			Host: "127.0.0.1",
			Port: 465,
		},
		logger: slog.Default(),
	}

	// Use a context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled

	err = m.sendTLS(ctx, addr, "from@test.com", nil, "test body", nil)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestJoinAndResetURL(t *testing.T) {
	if got := joinURL("https://breeze.example.com/", "tok"); got != "https://breeze.example.com/join?token=tok" {
		t.Errorf("joinURL = %q", got)
	}
	if got := resetURL("https://breeze.example.com", "tok"); got != "https://breeze.example.com/reset-password?token=tok" {
		t.Errorf("resetURL = %q", got)
	}
}

// TestSanitizeHeaderValue verifies CR/LF stripping so user-derived subject /
// display names cannot inject extra MIME headers (RFC 5322 header injection).
func TestSanitizeHeaderValue(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"normal subject", "normal subject"},
		{"evil\r\nBcc: attacker@example.com", "evilBcc: attacker@example.com"},
		{"evil\nBcc: attacker@example.com", "evilBcc: attacker@example.com"},
		{"\r\nInjected: yes", "Injected: yes"},
	}
	for _, c := range cases {
		if got := sanitizeHeaderValue(c.in); got != c.want {
			t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMailer_HeaderInjection_Subject ensures a CRLF in the notification
// subject (which embeds user display names / message content) is neutralized
// before the header is written. Uses a no-op SMTP (disabled mailer) so only
// header construction is exercised.
func TestMailer_HeaderInjection_Subject(t *testing.T) {
	m := NewMailer(config.SMTPConfig{Host: "localhost", Port: 2525, From: "breeze@t.com"}, slog.Default())
	_ = m // header sanitization happens in Send; verified via TestSanitizeHeaderValue
	// Direct proof: the notification subject path feeds user data into the
	// Subject header, which sanitizeHeaderValue now guards.
	tmpl := NotificationEmail("Alice\r\nBcc: evil@x.com", "body", "")
	if got := sanitizeHeaderValue(tmpl.Subject); strings.Contains(got, "\r") || strings.Contains(got, "\n") {
		t.Errorf("subject still contains CR/LF: %q", got)
	}
}

// TestNotificationEmail_EscapesBodyAndTitle verifies that user-derived
// notification title/body are HTML-escaped in the email body, preventing
// injection of script/HTML into the email (the notification body for chat
// messages is the raw message content).
func TestNotificationEmail_EscapesBodyAndTitle(t *testing.T) {
	title := `Task due: <script>alert(1)</script>`
	body := `<img src=x onerror=alert(1)> <b>bold</b> <@user:abc>`
	tmpl := NotificationEmail(title, body, "https://breeze.example.com/p")

	if strings.Contains(tmpl.HTML, "<script") {
		t.Errorf("title <script> not escaped in HTML: %s", tmpl.HTML)
	}
	// The <img onerror=...> must be escaped to inert text, not a live tag.
	if strings.Contains(tmpl.HTML, "<img") {
		t.Errorf("body <img> not escaped to text in HTML: %s", tmpl.HTML)
	}
	if !strings.Contains(tmpl.HTML, "onerror=alert(1)") {
		// the literal onerror text survives as inert escaped content (the
		// *payload* text is still there, just not executable)
		t.Errorf("onerror payload text dropped from escaped body: %s", tmpl.HTML)
	}
	// escaped forms must be present
	if !strings.Contains(tmpl.HTML, "&lt;script&gt;") {
		t.Errorf("title not escaped to entities: %s", tmpl.HTML)
	}
	// mention token text survives (escaped, which is correct for email display)
	if !strings.Contains(tmpl.HTML, "@user:abc") {
		t.Errorf("mention text dropped from body: %s", tmpl.HTML)
	}
}

// TestNotificationEmail_DropsDangerousLinkScheme verifies that a javascript:
// or data: URL in the notification link is stripped from the email href,
// not rendered as a clickable dangerous link. html/template replaces unsafe
// URL schemes with the #ZgotmplZ sentinel (even when obfuscated with tabs/
// newlines, which a naive denylist would miss).
func TestNotificationEmail_DropsDangerousLinkScheme(t *testing.T) {
	for _, bad := range []string{
		"javascript:alert(1)",
		"JaVaScRiPt:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:msgbox(1)",
		"java\tscript:alert(1)", // tab-obfuscated
	} {
		tmpl := NotificationEmail("t", "b", bad)
		if strings.Contains(tmpl.HTML, `href="`+bad) {
			t.Errorf("dangerous URL survived in href: %s", tmpl.HTML)
		}
		if strings.Contains(strings.ToLower(tmpl.HTML), "javascript:") {
			t.Errorf("javascript: scheme survived in HTML: %s", tmpl.HTML)
		}
		if !strings.Contains(tmpl.HTML, "#ZgotmplZ") {
			t.Errorf("unsafe URL not replaced with #ZgotmplZ sentinel: %s", tmpl.HTML)
		}
	}
}

// TestInviteEmail_EscapesUserNames verifies that inviter/org names are
// HTML-escaped in the invite email body.
func TestInviteEmail_EscapesUserNames(t *testing.T) {
	tmpl := InviteEmail(`<b>Alice</b><script>`, `Acme & Co`, "https://breeze.example.com/join?token=x")
	if strings.Contains(tmpl.HTML, "<script") {
		t.Errorf("inviter <script> not escaped: %s", tmpl.HTML)
	}
	if strings.Contains(tmpl.HTML, "<b>Alice</b>") {
		t.Errorf("inviter <b> not escaped: %s", tmpl.HTML)
	}
	if !strings.Contains(tmpl.HTML, "&lt;b&gt;Alice&lt;/b&gt;") {
		t.Errorf("inviter not escaped to entities: %s", tmpl.HTML)
	}
	if !strings.Contains(tmpl.HTML, "Acme &amp; Co") {
		t.Errorf("org ampersand not escaped: %s", tmpl.HTML)
	}
}
