package service

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	texttemplate "text/template"
)

// EmailTemplate holds the rendered subject + bodies for a transactional email.
type EmailTemplate struct {
	Subject string
	HTML    string
	Text    string
}

// Email templates live in templates/ and are embedded into the binary so a
// deployment never depends on the source tree. They are parsed once at package
// init and reused; never per call.
//
// HTML bodies use html/template, which provides context-aware auto-escaping:
// text content is HTML-escaped, and URLs in href/src contexts get their own
// escaping (dangerous schemes like javascript:/data:/vbscript: are replaced
// with the #ZgotmplZ sentinel). This prevents HTML/script injection when
// user-derived content (e.g. a chat message body for chat-DM notifications,
// inviter/org names, reset links) is interpolated; no manual
// html.EscapeString / safeURL needed.
//
// Plain-text bodies use text/template: there is no HTML context, so no
// escaping is applied; matching the raw-text contract of the text/plain MIME
// part. Subjects are built with fmt.Sprintf (a single plain-text line; the
// mailer strips CR/LF from header values, so no header injection is possible).

//go:embed templates
var emailTemplateFS embed.FS

// Shared HTML layout (docstart/docend) is in base.html; each email defines a
// root template named after its file (reset.html, invite.html,
// notification.html) that composes the layout.
var (
	htmlTmpl = template.Must(template.New("mail").ParseFS(emailTemplateFS, "templates/*.html"))
	textTmpl = texttemplate.Must(texttemplate.New("mail-text").ParseFS(emailTemplateFS, "templates/*.txt"))
)

type resetEmailData struct {
	URL string
}

type inviteEmailData struct {
	Org     string
	Inviter string
	URL     string
}

type notificationEmailData struct {
	Title string
	Body  string
	Link  string
}

// execHTML executes a named html/template. Errors are logged (never silently
// swallowed) and "" is returned so callers still produce a usable EmailTemplate.
// In practice execution cannot fail: templates are validated by template.Must
// at init and data structs are statically typed.
func execHTML(name string, data any) string {
	var b bytes.Buffer
	if err := htmlTmpl.ExecuteTemplate(&b, name, data); err != nil {
		slog.Error("render html email template", "template", name, "error", err)
		return ""
	}
	return b.String()
}

// execText executes a named text/template, same error policy as execHTML.
func execText(name string, data any) string {
	var b bytes.Buffer
	if err := textTmpl.ExecuteTemplate(&b, name, data); err != nil {
		slog.Error("render text email template", "template", name, "error", err)
		return ""
	}
	return b.String()
}

// PasswordResetEmail builds the password reset email. resetURL is the full
// link the recipient clicks (e.g. https://plume.example.com/reset-password?token=...).
func PasswordResetEmail(resetURL string) EmailTemplate {
	return EmailTemplate{
		Subject: "Reset your Plume password",
		Text:    execText("reset.txt", resetEmailData{URL: resetURL}),
		HTML:    execHTML("reset.html", resetEmailData{URL: resetURL}),
	}
}

// InviteEmail builds the invite email. joinURL is the full link the invitee
// clicks to accept (e.g. https://plume.example.com/join?token=...).
func InviteEmail(inviterName, orgName, joinURL string) EmailTemplate {
	inviter := inviterName
	if inviter == "" {
		inviter = "Someone"
	}
	org := orgName
	if org == "" {
		org = "a Plume workspace"
	}
	return EmailTemplate{
		// Subject is an email header (plain text), not HTML.
		Subject: fmt.Sprintf("%s invited you to join %s", inviter, org),
		Text:    execText("invite.txt", inviteEmailData{Org: org, Inviter: inviter, URL: joinURL}),
		HTML:    execHTML("invite.html", inviteEmailData{Org: org, Inviter: inviter, URL: joinURL}),
	}
}

// NotificationEmail builds a single-notification email. link is the in-app
// URL the recipient can click to view the relevant entity.
func NotificationEmail(title, body, link string) EmailTemplate {
	return EmailTemplate{
		Subject: title, // plain text header
		Text:    execText("notification.txt", notificationEmailData{Title: title, Body: body, Link: link}),
		HTML:    execHTML("notification.html", notificationEmailData{Title: title, Body: body, Link: link}),
	}
}

// joinURL builds a full invite-accept URL from a base app URL and token.
func joinURL(appURL, token string) string {
	return strings.TrimRight(appURL, "/") + "/join?token=" + token
}

// resetURL builds a full password-reset URL from a base app URL and token.
func resetURL(appURL, token string) string {
	return strings.TrimRight(appURL, "/") + "/reset-password?token=" + token
}
