// Package mailer sends mail over SMTP, reads the inbox over IMAP,
// and sends publish-event notifications. The mail password from
// the settings is used only to authenticate and is never logged
// or printed.
package mailer

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
)

// Message is one inbox message's envelope summary.
type Message struct {
	UID     uint32
	From    string
	Subject string
	Date    time.Time
}

// Send sends a plain-text email to one recipient using the SMTP
// settings in s. Port 465 uses implicit TLS; other ports use a
// plain connection upgraded with STARTTLS when the server
// advertises it.
func Send(s config.Settings, to, subject, body string) error {
	if s.Mail.SMTPHost == "" {
		return fmt.Errorf("SMTP is not configured (set mail settings in config.json)")
	}
	message := buildMessage(s.Mail.Username, to, subject, body, time.Now())
	client, err := dialSMTP(s)
	if err != nil {
		return err
	}
	defer client.Close()
	if s.Mail.Username != "" {
		auth := smtp.PlainAuth("", s.Mail.Username, s.Mail.Password, s.Mail.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err := client.Mail(s.Mail.Username); err != nil {
		return fmt.Errorf("SMTP MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("SMTP QUIT: %w", err)
	}
	return nil
}

// buildMessage renders a plain-text RFC 822 message.
func buildMessage(from, to, subject, body string, date time.Time) string {
	var builder strings.Builder
	builder.WriteString("From: " + from + "\r\n")
	builder.WriteString("To: " + to + "\r\n")
	builder.WriteString("Subject: " + subject + "\r\n")
	builder.WriteString("Date: " + date.Format(time.RFC1123Z) + "\r\n")
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	builder.WriteString("\r\n")
	builder.WriteString(body)
	return builder.String()
}

// dialSMTP connects to the configured SMTP server, using implicit
// TLS on port 465 and STARTTLS (when advertised) on other ports.
func dialSMTP(s config.Settings) (*smtp.Client, error) {
	addr := fmt.Sprintf("%s:%d", s.Mail.SMTPHost, s.Mail.SMTPPort)
	if s.Mail.SMTPPort == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.Mail.SMTPHost})
		if err != nil {
			return nil, fmt.Errorf("dial SMTP over TLS: %w", err)
		}
		client, err := smtp.NewClient(conn, s.Mail.SMTPHost)
		if err != nil {
			return nil, fmt.Errorf("create SMTP client: %w", err)
		}
		return client, nil
	}
	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("dial SMTP: %w", err)
	}
	if hasStartTLS, _ := client.Extension("STARTTLS"); hasStartTLS {
		if err := client.StartTLS(&tls.Config{ServerName: s.Mail.SMTPHost}); err != nil {
			client.Close()
			return nil, fmt.Errorf("SMTP STARTTLS: %w", err)
		}
	}
	return client, nil
}
