package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"horizon/foundation/system/governance"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func generateBoundary() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return "=_mail-" + hex.EncodeToString(data[:]), nil
}

func headerSafe(values ...string) bool {
	return !strings.ContainsAny(strings.Join(values, ""), "\r\n\x00")
}

type smtpEndpoint struct {
	Host, Username, Password, TLSMode string
	Port                              int
}

type outboundAttachment struct {
	Filename, MIME string
	Data           []byte
}

type outboundRecipient struct {
	Kind    string
	Address mail.Address
}

func allowedEndpoint(host string, port int, allowed []string) bool {
	target := net.JoinHostPort(strings.ToLower(strings.TrimSpace(host)), strconv.Itoa(port))
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, target) {
			return true
		}
	}
	return false
}

func safeAddress(raw string) (string, error) {
	if strings.ContainsAny(raw, "\r\n") {
		return "", governance.Problem("invalid")
	}
	address, err := mail.ParseAddress(raw)
	if err != nil || address.Address == "" {
		return "", governance.Problem("invalid")
	}
	return address.Address, nil
}

func smtpConnect(ctx context.Context, e smtpEndpoint, allowed []string) (*smtp.Client, error) {
	if !allowedEndpoint(e.Host, e.Port, allowed) {
		return nil, governance.Problem("forbidden")
	}
	if e.TLSMode != "starttls" && e.TLSMode != "tls" {
		return nil, governance.Problem("invalid")
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint(e.Host, e.Port))
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(20 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	if err = conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, err
	}
	if e.TLSMode == "tls" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: e.Host, MinVersion: tls.VersionTLS12})
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, e.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	fail := func(err error) (*smtp.Client, error) { client.Close(); return nil, err }
	if e.TLSMode == "starttls" {
		supported, _ := client.Extension("STARTTLS")
		if !supported {
			return fail(errors.New("smtp_tls_required"))
		}
		if err = client.StartTLS(&tls.Config{ServerName: e.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fail(err)
		}
	}
	if e.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", e.Username, e.Password, e.Host)); err != nil {
			return fail(err)
		}
	}
	return client, nil
}

func encodeBody(writer *multipart.Writer, contentType, body string) error {
	part, err := writer.CreatePart(map[string][]string{
		"Content-Type":              {contentType + "; charset=UTF-8"},
		"Content-Transfer-Encoding": {"base64"},
	})
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		if _, err = part.Write([]byte(encoded[:76] + "\r\n")); err != nil {
			return err
		}
		encoded = encoded[76:]
	}
	_, err = part.Write([]byte(encoded + "\r\n"))
	return err
}

func sendSMTP(ctx context.Context, e smtpEndpoint, allowed []string, from string, recipients []outboundRecipient, subject, textBody, htmlBody, messageID, replyTo string, attachments []outboundAttachment) error {
	if !headerSafe(subject, messageID, replyTo) {
		return governance.Problem("invalid")
	}
	sender, err := safeAddress(from)
	if err != nil {
		return err
	}
	envelope := make([]string, 0, len(recipients))
	headerTo := make([]string, 0, len(recipients))
	headerCc := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		address, err := safeAddress(recipient.Address.Address)
		if err != nil {
			return err
		}
		envelope = append(envelope, address)
		switch recipient.Kind {
		case "to":
			headerTo = append(headerTo, recipient.Address.String())
		case "cc":
			headerCc = append(headerCc, recipient.Address.String())
		case "bcc":
			// BCC participates in the envelope only and must not leak in headers.
		default:
			return governance.Problem("invalid")
		}
	}
	if len(envelope) == 0 {
		return governance.Problem("invalid")
	}
	replyHeader := ""
	if replyTo != "" {
		replyAddress, err := safeAddress(replyTo)
		if err != nil {
			return err
		}
		replyHeader = fmt.Sprintf("Reply-To: %s\r\n", (&mail.Address{Address: replyAddress}).String())
	}
	ccHeader := ""
	if len(headerCc) > 0 {
		ccHeader = fmt.Sprintf("Cc: %s\r\n", strings.Join(headerCc, ", "))
	}
	client, err := smtpConnect(ctx, e, allowed)
	if err != nil {
		return err
	}
	defer client.Close()
	if err = client.Mail(sender); err != nil {
		return err
	}
	for _, recipient := range envelope {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	parts := multipart.NewWriter(writer)
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\n%sSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@baseline.local>\r\n%sMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n",
		(&mail.Address{Address: sender}).String(), strings.Join(headerTo, ", "), ccHeader, mime.QEncoding.Encode("utf-8", subject),
		time.Now().Format(time.RFC1123Z), url.QueryEscape(messageID), replyHeader, parts.Boundary())
	if _, err = writer.Write([]byte(headers)); err != nil {
		return err
	}
	boundary, err := generateBoundary()
	if err != nil {
		return err
	}
	alternativePart, err := parts.CreatePart(map[string][]string{
		"Content-Type": {"multipart/alternative; boundary=" + boundary},
	})
	if err != nil {
		return err
	}
	alternative := multipart.NewWriter(alternativePart)
	if err = alternative.SetBoundary(boundary); err != nil {
		return err
	}
	if err = encodeBody(alternative, "text/plain", textBody); err != nil {
		return err
	}
	if err = encodeBody(alternative, "text/html", htmlBody); err != nil {
		return err
	}
	if err = alternative.Close(); err != nil {
		return err
	}
	for _, attachment := range attachments {
		asciiName := mime.QEncoding.Encode("utf-8", strings.ReplaceAll(attachment.Filename, `"`, ""))
		part, err := parts.CreatePart(map[string][]string{
			"Content-Type":              {attachment.MIME + `; name="` + asciiName + `"`},
			"Content-Disposition":       {`attachment; filename="` + asciiName + `"`},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return err
		}
		encoded := base64.StdEncoding.EncodeToString(attachment.Data)
		for len(encoded) > 76 {
			if _, err = part.Write([]byte(encoded[:76] + "\r\n")); err != nil {
				return err
			}
			encoded = encoded[76:]
		}
		if _, err = part.Write([]byte(encoded + "\r\n")); err != nil {
			return err
		}
	}
	if err = parts.Close(); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
