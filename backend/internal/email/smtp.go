package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type SMTPMailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// ImplicitTLS forces TLS on connect (typical for port 465).
	ImplicitTLS bool
}

func (m SMTPMailer) Name() string { return "SMTP" }

func (m SMTPMailer) Verify(ctx context.Context) error {
	client, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Quit()
}

func (m SMTPMailer) Send(ctx context.Context, message Message) (SendResult, error) {
	if !looksLikeEmail(message.ToEmail) || strings.TrimSpace(message.Subject) == "" {
		return SendResult{}, errValidation
	}
	if strings.TrimSpace(message.TextBody) == "" && strings.TrimSpace(message.HtmlBody) == "" {
		return SendResult{}, errValidation
	}
	client, err := m.dial(ctx)
	if err != nil {
		return SendResult{}, err
	}
	defer client.Close()

	from := firstNonEmpty(strings.TrimSpace(message.FromEmail), m.From)
	if err := client.Mail(from); err != nil {
		return SendResult{}, fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(message.ToEmail); err != nil {
		return SendResult{}, fmt.Errorf("smtp RCPT TO: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return SendResult{}, fmt.Errorf("smtp DATA: %w", err)
	}
	payload := buildSMTPMessage(message)
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return SendResult{}, fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return SendResult{}, fmt.Errorf("smtp close: %w", err)
	}
	_ = client.Quit()
	return SendResult{
		MessageID: "smtp_" + time.Now().UTC().Format("20060102T150405.000000000"),
		Status:    StatusCaptured,
	}, nil
}

func (m SMTPMailer) dial(ctx context.Context) (*smtp.Client, error) {
	host := strings.TrimSpace(m.Host)
	port := m.Port
	if host == "" {
		return nil, errors.New("SMTP host is required")
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("SMTP port is invalid")
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	deadline := 20 * time.Second
	if due, ok := ctx.Deadline(); ok {
		deadline = time.Until(due)
		if deadline < 2*time.Second {
			deadline = 2 * time.Second
		}
	}

	var (
		conn net.Conn
		err  error
	)
	dialer := &net.Dialer{Timeout: deadline}
	if m.ImplicitTLS || port == 465 {
		tlsConfig := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("smtp connect %s: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(deadline))

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smtp client: %w", err)
	}

	if !m.ImplicitTLS && port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
			if err := client.StartTLS(tlsConfig); err != nil {
				_ = client.Close()
				return nil, fmt.Errorf("smtp STARTTLS: %w", err)
			}
		}
	}

	username := strings.TrimSpace(m.Username)
	if username != "" {
		auth := smtp.PlainAuth("", username, m.Password, host)
		if err := client.Auth(auth); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("smtp auth: %w", err)
		}
	}
	return client, nil
}

func buildSMTPMessage(message Message) []byte {
	fromName := strings.TrimSpace(message.FromName)
	fromEmail := strings.TrimSpace(message.FromEmail)
	toName := strings.TrimSpace(message.ToName)
	toEmail := strings.TrimSpace(message.ToEmail)
	replyTo := strings.TrimSpace(message.ReplyTo)
	subject := strings.TrimSpace(message.Subject)
	textBody := strings.ReplaceAll(message.TextBody, "\r\n", "\n")
	htmlBody := strings.TrimSpace(message.HtmlBody)

	var b strings.Builder
	b.WriteString("From: " + formatAddress(fromName, fromEmail) + "\r\n")
	b.WriteString("To: " + formatAddress(toName, toEmail) + "\r\n")
	if replyTo != "" {
		b.WriteString("Reply-To: " + replyTo + "\r\n")
	}
	b.WriteString("Subject: " + encodeSMTPHeader(subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")

	if htmlBody == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(strings.ReplaceAll(textBody, "\n", "\r\n"))
		b.WriteString("\r\n")
		return []byte(b.String())
	}

	boundary := "chakuchuri_" + time.Now().UTC().Format("20060102150405")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(strings.ReplaceAll(textBody, "\n", "\r\n"))
	b.WriteString("\r\n--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(htmlBody)
	b.WriteString("\r\n--" + boundary + "--\r\n")
	return []byte(b.String())
}

func formatAddress(name, email string) string {
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	if name == "" {
		return email
	}
	address := mail.Address{Name: name, Address: email}
	return address.String()
}

func encodeSMTPHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
