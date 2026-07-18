package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
)

type EmailNotifier struct {
	host     string
	port     int
	username string
	password string
	from     string
	to       string
	logger   *slog.Logger
}

func NewEmailNotifier(host string, port int, username, password, from, to string, logger *slog.Logger) *EmailNotifier {
	return &EmailNotifier{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		to:       to,
		logger:   logger,
	}
}

func (n *EmailNotifier) Enabled() bool {
	return n.host != ""
}

func (n *EmailNotifier) addr() string {
	return net.JoinHostPort(n.host, strconv.Itoa(n.port))
}

func (n *EmailNotifier) Send(ctx context.Context, commentID int64) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", n.addr())
	if err != nil {
		return fmt.Errorf("connecting to SMTP server: %w", err)
	}

	client, err := smtp.NewClient(conn, n.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("creating SMTP client: %w", err)
	}
	defer client.Close()

	if n.username != "" && n.password != "" {
		auth := smtp.PlainAuth("", n.username, n.password, n.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	if err := client.Mail(n.from); err != nil {
		return fmt.Errorf("SMTP mail from: %w", err)
	}
	if err := client.Rcpt(n.to); err != nil {
		return fmt.Errorf("SMTP rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP data: %w", err)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: New comment created (#%d)\r\n\r\nA new comment has been submitted and requires moderation.\r\n\r\nComment ID: %d\r\n",
		n.from, n.to, commentID, commentID)

	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("writing email body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("closing email body: %w", err)
	}

	return client.Quit()
}
