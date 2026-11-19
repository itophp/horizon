package mail

import "testing"

func TestMailOptionsDefaultsAndValidation(t *testing.T) {
	t.Setenv("MAIL_ALLOWED_ENDPOINTS", "smtp.example.test:583, pop.example.test:995")
	t.Setenv("MAIL_MAX_MESSAGE_BYTES", "26214400")
	t.Setenv("MAIL_SYNC_BATCH", "20")
	t.Setenv("MAIL_RETRY_MAX", "5")
	options, err := OptionsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if len(options.AllowedEndpoints) != 2 || options.AllowedEndpoints[0] != "smtp.example.test:583" || options.AllowedEndpoints[1] != "pop.example.test:995" {
		t.Fatalf("mail endpoints = %v", options.AllowedEndpoints)
	}
	if options.MaxMessageBytes != 26214400 || options.SyncBatch != 20 || options.RetryMax != 5 {
		t.Fatalf("mail limits = %+v", options)
	}
	t.Setenv("MAIL_ALLOWED_ENDPOINTS", "https://example.test")
	if _, err := OptionsFromEnv(); err == nil {
		t.Fatal("endpoint without an explicit port was accepted")
	}
	t.Setenv("MAIL_ALLOWED_ENDPOINTS", "smtp.example.test:587")
	t.Setenv("MAIL_MAX_MESSAGE_BYTES", "0")
	if _, err := OptionsFromEnv(); err == nil {
		t.Fatal("zero message limit was accepted")
	}
}

func TestMailOptionsUseSafeProductionDefaults(t *testing.T) {
	for _, name := range []string{
		"MAIL_ALLOWED_ENDPOINTS", "MAIL_MAX_MESSAGE_BYTES", "MAIL_MAX_ATTACHMENT_BYTES", "MAIL_MAX_ATTACHMENTS",
		"MAIL_SEND_MAX_RECIPIENTS", "MAIL_SYNC_BATCH", "MAIL_RETRY_MAX",
	} {
		t.Setenv(name, "")
	}
	options, err := OptionsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if options.MaxMessageBytes != 26214400 ||
		options.MaxAttachmentBytes != 10485760 ||
		options.MaxAttachments != 20 ||
		options.SendMaxRecipients != 200 ||
		options.SyncBatch != 20 ||
		options.RetryMax != 5 {
		t.Fatalf("mail defaults = %+v", options)
	}
}
