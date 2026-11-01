package notifications

import (
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type smtpConfig struct {
	Host, User, Password, From, TLS string
	Port                            int
	Enabled                         bool
}

func (s *Service) smtpConfiguration(ctx context.Context, p *identity.Principal) (smtpConfig, error) {
	out := smtpConfig{}
	for key, dest := range map[string]*string{"smtp.host": &out.Host, "smtp.username": &out.User, "smtp.password": &out.Password, "smtp.from": &out.From, "smtp.tls": &out.TLS} {
		raw, err := s.Setting(ctx, p, key)
		if err != nil {
			return out, err
		}
		*dest, err = governance.String(raw)
		if err != nil {
			return out, err
		}
	}
	raw, err := s.Setting(ctx, p, "smtp.port")
	if err != nil {
		return out, err
	}
	n, err := governance.Number(raw)
	out.Port = int(n)
	if err != nil {
		return out, err
	}
	raw, err = s.Setting(ctx, p, "smtp.enabled")
	if err != nil {
		return out, err
	}
	out.Enabled, err = governance.Bool(raw)
	return out, err
}

// SMTP always verifies certificates. The plaintext exception is limited to an explicit
// isolated-test option and a literal loopback host; it cannot be set from settings.
func (s *Service) smtpConnect(ctx context.Context, c smtpConfig) (*smtp.Client, error) {
	if !c.Enabled || !governance.Has(s.Options.SMTPHosts, c.Host) {
		return nil, governance.Problem("unavailable")
	}
	test := s.Options.AllowTestSMTP && net.ParseIP(c.Host) != nil && net.ParseIP(c.Host).IsLoopback()
	if c.TLS != "tls" && c.TLS != "starttls" && !test {
		return nil, governance.Problem("invalid")
	}
	address := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(15 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err = conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, err
	}
	config := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	if c.TLS == "tls" {
		tlsConn := tls.Client(conn, config)
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	fail := func(e error) (*smtp.Client, error) { client.Close(); return nil, e }
	if c.TLS == "starttls" {
		ok, _ := client.Extension("STARTTLS")
		if !ok && !test {
			return fail(errors.New("smtp_tls_required"))
		}
		if ok {
			if err = client.StartTLS(config); err != nil {
				return fail(err)
			}
		}
	}
	if c.User != "" {
		if err = client.Auth(smtp.PlainAuth("", c.User, c.Password, c.Host)); err != nil {
			return fail(err)
		}
	}
	return client, nil
}
func safeAddress(raw string) (string, error) {
	if strings.ContainsAny(raw, "\r\n") {
		return "", governance.Problem("invalid")
	}
	a, err := mail.ParseAddress(raw)
	if err != nil {
		return "", err
	}
	return a.Address, nil
}
func (s *Service) send(ctx context.Context, c smtpConfig, to, subject, body, id string, send bool) error {
	from, err := safeAddress(c.From)
	if err != nil {
		return err
	}
	recipient, err := safeAddress(to)
	if err != nil {
		return err
	}
	if strings.ContainsAny(subject, "\r\n") {
		return governance.Problem("invalid")
	}
	client, err := s.smtpConnect(ctx, c)
	if err != nil {
		return err
	}
	defer client.Close()
	if !send {
		return client.Quit()
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	if err = client.Rcpt(recipient); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	message := fmt.Sprintf("From: <%s>\r\nTo: <%s>\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@baseline.local>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n", from, recipient, mime.BEncoding.Encode("utf-8", subject), time.Now().Format(time.RFC1123Z), id)
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		message += encoded[:76] + "\r\n"
		encoded = encoded[76:]
	}
	message += encoded + "\r\n"
	if _, err = writer.Write([]byte(message)); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
