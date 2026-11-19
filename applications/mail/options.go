package mail

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
)

// Options are the business limits of the mail module. Deployment
// configuration resolves them into the plugin; the platform outbound policy
// (TLS, SSRF, budgets) stays a kernel concern.
type Options struct {
	MaxMessageBytes    int
	MaxAttachmentBytes int
	MaxAttachments     int
	SendMaxRecipients  int
	SyncBatch          int
	RetryMax           int
	// AllowedEndpoints lists the host:port endpoints the module may reach for
	// SMTP and POP3 traffic.
	AllowedEndpoints []string
}

// defaults applies the documented limits when the deployment does not
// override them.
func (o Options) defaults() Options {
	if o.MaxMessageBytes == 0 {
		o.MaxMessageBytes = 26214400
	}
	if o.MaxAttachmentBytes == 0 {
		o.MaxAttachmentBytes = 10485760
	}
	if o.MaxAttachments == 0 {
		o.MaxAttachments = 20
	}
	if o.SendMaxRecipients == 0 {
		o.SendMaxRecipients = 200
	}
	if o.SyncBatch == 0 {
		o.SyncBatch = 20
	}
	if o.RetryMax == 0 {
		o.RetryMax = 5
	}
	return o
}

// OptionsFromEnv resolves the mail options from deployment environment
// variables, keeping the documented validation ranges.
func OptionsFromEnv() (Options, error) {
	options := Options{}
	for _, endpoint := range strings.Split(os.Getenv("MAIL_ALLOWED_ENDPOINTS"), ",") {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" {
			continue
		}
		host, port, err := net.SplitHostPort(endpoint)
		if err != nil || host == "" || port == "" || strings.ContainsAny(endpoint, " \t\r\n/") {
			return options, errors.New("MAIL_ALLOWED_ENDPOINTS must be a comma-separated host:port list")
		}
		options.AllowedEndpoints = append(options.AllowedEndpoints, net.JoinHostPort(host, port))
	}
	integers := []struct {
		name         string
		value        *int
		min          int
		max          int
		defaultValue int
	}{
		{"MAIL_MAX_MESSAGE_BYTES", &options.MaxMessageBytes, 1024, 26214400, 26214400},
		{"MAIL_MAX_ATTACHMENT_BYTES", &options.MaxAttachmentBytes, 1, 10485760, 10485760},
		{"MAIL_MAX_ATTACHMENTS", &options.MaxAttachments, 1, 20, 20},
		{"MAIL_SEND_MAX_RECIPIENTS", &options.SendMaxRecipients, 1, 200, 200},
		{"MAIL_SYNC_BATCH", &options.SyncBatch, 1, 100, 20},
		{"MAIL_RETRY_MAX", &options.RetryMax, 1, 10, 5},
	}
	for _, item := range integers {
		raw := strings.TrimSpace(os.Getenv(item.name))
		if raw == "" {
			*item.value = item.defaultValue
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < item.min || value > item.max {
			return options, errors.New(item.name + " is outside the allowed range")
		}
		*item.value = value
	}
	return options, nil
}
