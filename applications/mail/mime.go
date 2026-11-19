package mail

import (
	"bytes"
	"errors"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"
)

type parsedAttachment struct {
	ContentID   string
	Filename    string
	MIME        string
	Disposition string
	Data        []byte
}

var ErrMimeTooLarge = errors.New("mail message exceeds the configured size limit")

type parsedMessage struct {
	RFCMessageID string
	Subject      string
	From         string
	ReplyTo      string
	To           []mail.Address
	CC           []mail.Address
	Text         string
	HTML         string
	OccurredAt   time.Time
	Attachments  []parsedAttachment
}

func readLimited(reader io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, ErrMimeTooLarge
	}
	return data, nil
}

func parseAddressHeader(value string) []mail.Address {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := mail.ParseAddressList(value)
	if err != nil {
		return nil
	}
	out := make([]mail.Address, 0, len(parsed))
	for _, address := range parsed {
		out = append(out, *address)
	}
	return out
}

func firstAddress(value string) string {
	addresses := parseAddressHeader(value)
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0].Address
}

func ParseMIME(entity *message.Entity, maxBytes int) (*parsedMessage, error) {
	out := &parsedMessage{OccurredAt: time.Now(), Attachments: []parsedAttachment{}}
	out.RFCMessageID = strings.TrimSpace(entity.Header.Get("Message-Id"))
	out.Subject = strings.TrimSpace(entity.Header.Get("Subject"))
	out.From = firstAddress(entity.Header.Get("From"))
	out.ReplyTo = firstAddress(entity.Header.Get("Reply-To"))
	out.To = parseAddressHeader(entity.Header.Get("To"))
	out.CC = parseAddressHeader(entity.Header.Get("Cc"))
	if date := strings.TrimSpace(entity.Header.Get("Date")); date != "" {
		if parsed, err := mail.ParseDate(date); err == nil {
			out.OccurredAt = parsed
		}
	}
	textBody, htmlBody := "", ""
	totalBytes := 0
	err := entity.Walk(func(_ []int, part *message.Entity, walkErr error) error {
		if walkErr != nil && !message.IsUnknownCharset(walkErr) && !message.IsUnknownEncoding(walkErr) {
			return walkErr
		}
		mediaType, params, _ := part.Header.ContentType()
		disposition, dispositionParams, _ := part.Header.ContentDisposition()
		filename := strings.TrimSpace(dispositionParams["filename"])
		if filename == "" {
			filename = strings.TrimSpace(params["name"])
		}
		isAttachment := filename != "" || disposition == "attachment"
		if isAttachment {
			data, err := readLimited(part.Body, maxBytes)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return nil
			}
			totalBytes += len(data)
			if disposition != "inline" {
				disposition = "attachment"
			}
			out.Attachments = append(out.Attachments, parsedAttachment{
				ContentID: strings.Trim(strings.TrimSpace(part.Header.Get("Content-Id")), "<>"),
				Filename:  filename, MIME: mediaType, Disposition: disposition, Data: data,
			})
			return nil
		}
		switch {
		case strings.EqualFold(mediaType, "text/plain"):
			data, err := readLimited(part.Body, maxBytes)
			if err != nil {
				return err
			}
			textBody = string(data)
		case strings.EqualFold(mediaType, "text/html"):
			data, err := readLimited(part.Body, maxBytes)
			if err != nil {
				return err
			}
			htmlBody = string(data)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if totalBytes > maxBytes {
		return nil, ErrMimeTooLarge
	}
	safeHTML, err := SanitizeHTML(htmlBody, maxBytes)
	if err != nil {
		return nil, err
	}
	out.HTML = safeHTML
	out.Text = strings.ToValidUTF8(textBody, "")
	if out.Text == "" && safeHTML != "" {
		out.Text = HTMLToText(safeHTML)
	}
	return out, nil
}

func textprotoHeader(key, value string) textproto.Header {
	var header textproto.Header
	header.Add(key, value)
	return header
}

func messageReader(data []byte) *bytes.Reader { return bytes.NewReader(data) }
