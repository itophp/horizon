package mail

import (
	"context"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/governance"
	"horizon/internal/repository/dbgen"
)

func validateMailboxInput(in *OnboardInput) error {
	address, err := mail.ParseAddress(in.EmailAddress)
	if err != nil || address.Address == "" || strings.ContainsAny(in.EmailAddress, "\r\n") {
		return contracts.Problem("invalid")
	}
	in.EmailAddress = strings.ToLower(address.Address)
	if in.Password == "" || in.SMTPHost == "" || in.POP3Host == "" ||
		in.SMTPPort < 1 || in.SMTPPort > 65535 || in.POP3Port < 1 || in.POP3Port > 65535 ||
		(in.SMTPTLS != "starttls" && in.SMTPTLS != "tls") {
		return contracts.Problem("invalid")
	}
	if !headerSafe(in.Name, in.EmailAddress, in.SMTPHost, in.POP3Host, in.SMTPUsername, in.POP3Username) {
		return contracts.Problem("invalid")
	}
	if in.SMTPUsername == "" {
		in.SMTPUsername = in.EmailAddress
	}
	if in.POP3Username == "" {
		in.POP3Username = in.EmailAddress
	}
	return nil
}

func (m *Module) onboard(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := OnboardInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err = validateMailboxInput(&payload); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	existing, err := q.MailMailboxList(ctx, dbgen.MailMailboxListParams{TenantID: p.TenantID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, err
	}
	defaultSend := len(existing) == 0
	id, err := q.MailMailboxCreate(ctx, dbgen.MailMailboxCreateParams{
		TenantID: p.TenantID, OwnerAccountID: p.Account.ID, Name: payload.Name,
		EmailAddress: strings.ToLower(payload.EmailAddress), DefaultSend: defaultSend,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	smtpCiphertext, smtpFingerprint, err := sealCredential(m.Core, p.TenantID, id, "smtp", payload.Password)
	if err != nil {
		return contracts.Output{}, err
	}
	smtpCredential, err := q.MailCredentialCreate(ctx, dbgen.MailCredentialCreateParams{
		TenantID: p.TenantID, MailboxID: id, Purpose: "smtp", Ciphertext: smtpCiphertext, Fingerprint: smtpFingerprint,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	popCiphertext, popFingerprint, err := sealCredential(m.Core, p.TenantID, id, "pop3", payload.Password)
	if err != nil {
		return contracts.Output{}, err
	}
	popCredential, err := q.MailCredentialCreate(ctx, dbgen.MailCredentialCreateParams{
		TenantID: p.TenantID, MailboxID: id, Purpose: "pop3", Ciphertext: popCiphertext, Fingerprint: popFingerprint,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if _, err = q.MailChannelCreate(ctx, dbgen.MailChannelCreateParams{
		TenantID: p.TenantID, MailboxID: id, Direction: "send", Protocol: "smtp", Host: payload.SMTPHost,
		Port: int32(payload.SMTPPort), TlsMode: payload.SMTPTLS, Username: payload.SMTPUsername, CredentialID: smtpCredential.ID,
	}); err != nil {
		return contracts.Output{}, problem(err)
	}
	if _, err = q.MailChannelCreate(ctx, dbgen.MailChannelCreateParams{
		TenantID: p.TenantID, MailboxID: id, Direction: "receive", Protocol: "pop3", Host: payload.POP3Host,
		Port: int32(payload.POP3Port), TlsMode: "tls", Username: payload.POP3Username, CredentialID: popCredential.ID,
	}); err != nil {
		return contracts.Output{}, problem(err)
	}
	if err = m.testSMTPDirect(ctx, smtpEndpoint{
		Host: payload.SMTPHost, Port: payload.SMTPPort, TLSMode: payload.SMTPTLS,
		Username: payload.SMTPUsername, Password: payload.Password,
	}); err != nil {
		_, _ = q.MailChannelVerificationUpdate(ctx, dbgen.MailChannelVerificationUpdateParams{
			VerificationStatus: "failed", LastErrorCode: "unavailable", TenantID: p.TenantID, MailboxID: id, Direction: "send",
		})
		_, _ = q.MailChannelVerificationUpdate(ctx, dbgen.MailChannelVerificationUpdateParams{
			VerificationStatus: "failed", LastErrorCode: "untested", TenantID: p.TenantID, MailboxID: id, Direction: "receive",
		})
		return contracts.Output{Data: map[string]any{"id": id, "status": "unverified"},
			Events: []contracts.Event{mailEvent(MailboxOnboardedEvent, "mail_mailbox", id, map[string]any{
				"mailboxId": id, "status": "unverified",
			})}}, nil
	}
	if err = testPOP3(ctx, payload.POP3Host, payload.POP3Port, payload.POP3Username, payload.Password, m.options.AllowedEndpoints); err != nil {
		_, _ = q.MailChannelVerificationUpdate(ctx, dbgen.MailChannelVerificationUpdateParams{
			VerificationStatus: "failed", LastErrorCode: "unavailable", TenantID: p.TenantID, MailboxID: id, Direction: "receive",
		})
		return contracts.Output{Data: map[string]any{"id": id, "status": "unverified"},
			Events: []contracts.Event{mailEvent(MailboxOnboardedEvent, "mail_mailbox", id, map[string]any{
				"mailboxId": id, "status": "unverified",
			})}}, nil
	}
	rows, err := q.MailMailboxStatusUpdate(ctx, dbgen.MailMailboxStatusUpdateParams{
		Status: "active", TenantID: p.TenantID, ID: id, OwnerAccountID: p.Account.ID,
		Column5: []string{"unverified"},
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if _, err = q.MailChannelVerificationUpdate(ctx, dbgen.MailChannelVerificationUpdateParams{
		VerificationStatus: "ok", LastErrorCode: "", TenantID: p.TenantID, MailboxID: id, Direction: "send",
	}); err != nil {
		return contracts.Output{}, err
	}
	if _, err = q.MailChannelVerificationUpdate(ctx, dbgen.MailChannelVerificationUpdateParams{
		VerificationStatus: "ok", LastErrorCode: "", TenantID: p.TenantID, MailboxID: id, Direction: "receive",
	}); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data: map[string]any{"id": id, "status": "active"}, Summary: "mailbox=connected",
		Events: []contracts.Event{mailEvent(MailboxOnboardedEvent, "mail_mailbox", id, map[string]any{
			"mailboxId": id, "status": "active",
		})},
	}, nil
}

func (m *Module) testSMTPDirect(ctx context.Context, endpoint smtpEndpoint) error {
	client, err := smtpConnect(ctx, endpoint, m.options.AllowedEndpoints)
	if err != nil {
		return contracts.Problem("unavailable")
	}
	if err = client.Noop(); err != nil {
		client.Close()
		return contracts.Problem("unavailable")
	}
	if err = client.Quit(); err != nil {
		return contracts.Problem("unavailable")
	}
	return nil
}

func mailboxData(row dbgen.MailMailboxListRow, channels []dbgen.MailChannelsListRow) map[string]any {
	items := []map[string]any{}
	for _, channel := range channels {
		items = append(items, map[string]any{
			"direction": channel.Direction, "protocol": channel.Protocol, "host": channel.Host,
			"port": channel.Port, "tlsMode": channel.TlsMode, "username": channel.Username,
			"verificationStatus": channel.VerificationStatus, "lastErrorCode": channel.LastErrorCode,
			"lastVerifiedAt": governance.Time(channel.LastVerifiedAt),
			"credential":     map[string]any{"version": channel.Version, "fingerprint": channel.Fingerprint},
		})
	}
	return map[string]any{
		"id": row.ID, "name": row.Name, "emailAddress": row.EmailAddress,
		"defaultSend": row.DefaultSend, "status": row.Status, "revision": row.Revision,
		"createdAt": governance.Time(row.CreatedAt), "updatedAt": governance.Time(row.UpdatedAt), "channels": items,
	}
}

func (m *Module) listMailboxes(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	rows, err := m.Q.MailMailboxList(ctx, dbgen.MailMailboxListParams{TenantID: p.TenantID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	items := []map[string]any{}
	for _, row := range rows {
		channels, err := m.Q.MailChannelsList(ctx, dbgen.MailChannelsListParams{
			TenantID: p.TenantID, MailboxID: row.ID, OwnerAccountID: p.Account.ID,
		})
		if err != nil {
			return contracts.Output{}, problem(err)
		}
		items = append(items, mailboxData(row, channels))
	}
	return contracts.Output{Data: map[string]any{"items": items}, Summary: "mailboxes=" + string(rune(len(items)))}, nil
}

func (m *Module) getMailbox(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := MessageGetInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	row, err := m.Q.MailMailboxGet(ctx, dbgen.MailMailboxGetParams{TenantID: p.TenantID, ID: payload.ID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	channels, err := m.Q.MailChannelsList(ctx, dbgen.MailChannelsListParams{TenantID: p.TenantID, MailboxID: row.ID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	return contracts.Output{Data: mailboxData(dbgen.MailMailboxListRow(row), channels)}, nil
}

func (m *Module) updateMailbox(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := MailboxUpdateInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	address, err := mail.ParseAddress(payload.EmailAddress)
	if err != nil || address.Address == "" || strings.ContainsAny(payload.Name+payload.EmailAddress, "\r\n") {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	payload.EmailAddress = strings.ToLower(address.Address)
	q := dbgen.New(tx)
	if payload.DefaultSend {
		if _, err = q.MailMailboxDefaultClear(ctx, dbgen.MailMailboxDefaultClearParams{TenantID: p.TenantID, OwnerAccountID: p.Account.ID}); err != nil {
			return contracts.Output{}, err
		}
	}
	rows, err := q.MailMailboxUpdate(ctx, dbgen.MailMailboxUpdateParams{
		Name: payload.Name, EmailAddress: strings.ToLower(payload.EmailAddress), DefaultSend: payload.DefaultSend,
		TenantID: p.TenantID, ID: payload.ID, OwnerAccountID: p.Account.ID, Revision: payload.Revision,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	channels, err := q.MailChannelsList(ctx, dbgen.MailChannelsListParams{TenantID: p.TenantID, MailboxID: payload.ID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	for _, channel := range channels {
		host, port, tlsMode, username := channel.Host, channel.Port, channel.TlsMode, channel.Username
		if channel.Direction == "send" {
			host, port, tlsMode, username = payload.SMTPHost, int32(payload.SMTPPort), payload.SMTPTLS, payload.SMTPUsername
		} else {
			host, port, tlsMode, username = payload.POP3Host, int32(payload.POP3Port), "tls", payload.POP3Username
		}
		if _, err = q.MailChannelCreate(ctx, dbgen.MailChannelCreateParams{
			TenantID: p.TenantID, MailboxID: payload.ID, Direction: channel.Direction, Protocol: channel.Protocol,
			Host: host, Port: port, TlsMode: tlsMode, Username: username, CredentialID: channel.CredentialID,
		}); err != nil {
			return contracts.Output{}, problem(err)
		}
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID}, Events: []contracts.Event{
		mailEvent(MailboxUpdatedEvent, "mail_mailbox", payload.ID, map[string]any{"mailboxId": payload.ID}),
	}}, nil
}

func (m *Module) replaceCredential(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := CredentialInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if payload.Password == "" || (payload.Purpose != "smtp" && payload.Purpose != "pop3") {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	q := dbgen.New(tx)
	channels, err := q.MailChannelsList(ctx, dbgen.MailChannelsListParams{TenantID: p.TenantID, MailboxID: payload.MailboxID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	found := false
	credentialID := int64(0)
	nextVersion := int32(1)
	for _, channel := range channels {
		if (payload.Purpose == "smtp" && channel.Direction == "send") || (payload.Purpose == "pop3" && channel.Direction == "receive") {
			found, credentialID, nextVersion = true, channel.CredentialID, channel.Version+1
		}
	}
	if !found {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	ciphertext, err := m.Seal(p.TenantID, credentialScope(payload.MailboxID), credentialKey(payload.Purpose, nextVersion), payload.Password)
	if err != nil {
		return contracts.Output{}, err
	}
	rows, err := q.MailCredentialReplace(ctx, dbgen.MailCredentialReplaceParams{
		Ciphertext: ciphertext, Fingerprint: governance.Hash(ciphertext)[:16], TenantID: p.TenantID,
		MailboxID: payload.MailboxID, Purpose: payload.Purpose,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	direction := "send"
	if payload.Purpose == "pop3" {
		direction = "receive"
	}
	if _, err = q.MailChannelVerificationUpdate(ctx, dbgen.MailChannelVerificationUpdateParams{
		VerificationStatus: "unknown", LastErrorCode: "", TenantID: p.TenantID, MailboxID: payload.MailboxID, Direction: direction,
	}); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"mailboxId": payload.MailboxID, "purpose": payload.Purpose, "version": nextVersion},
		Events: []contracts.Event{mailEvent(CredentialRotated, "mail_credential", credentialID, map[string]any{
			"mailboxId": payload.MailboxID, "purpose": payload.Purpose,
		})}}, nil
}

func (m *Module) setMailboxStatus(ctx context.Context, tx pgx.Tx, in contracts.Input, status string, event string) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := MailboxStatusInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	allowed := []string{"active"}
	if status == "active" {
		allowed = []string{"disabled", "unverified"}
	}
	rows, err := dbgen.New(tx).MailMailboxStatusUpdate(ctx, dbgen.MailMailboxStatusUpdateParams{
		Status: status, TenantID: p.TenantID, ID: payload.ID, OwnerAccountID: p.Account.ID, Column5: allowed,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID, "status": status}, Events: []contracts.Event{
		mailEvent(event, "mail_mailbox", payload.ID, map[string]any{"mailboxId": payload.ID, "status": status}),
	}}, nil
}

func (m *Module) enableMailbox(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	return m.setMailboxStatus(ctx, tx, in, "active", MailboxEnabledEvent)
}

func (m *Module) disableMailbox(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	return m.setMailboxStatus(ctx, tx, in, "disabled", MailboxDisabledEvent)
}

func (m *Module) testConnection(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := ConnectionTestInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	channels, err := q.MailChannelsList(ctx, dbgen.MailChannelsListParams{TenantID: p.TenantID, MailboxID: payload.MailboxID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	for _, channel := range channels {
		if (payload.Direction == "send" && channel.Direction != "send") || (payload.Direction == "receive" && channel.Direction != "receive") {
			continue
		}
		purpose := "smtp"
		if channel.Direction == "receive" {
			purpose = "pop3"
		}
		password, err := openCredential(m.Core, p.TenantID, payload.MailboxID, purpose, channel.Version, channel.Ciphertext)
		if err != nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		if channel.Direction == "send" {
			err = m.testSMTPDirect(ctx, smtpEndpoint{Host: channel.Host, Port: int(channel.Port), TLSMode: channel.TlsMode, Username: channel.Username, Password: password})
		} else {
			err = testPOP3(ctx, channel.Host, int(channel.Port), channel.Username, password, m.options.AllowedEndpoints)
		}
		if err != nil {
			if _, updateErr := q.MailChannelVerificationUpdate(ctx, dbgen.MailChannelVerificationUpdateParams{
				VerificationStatus: "failed", LastErrorCode: "unavailable", TenantID: p.TenantID,
				MailboxID: payload.MailboxID, Direction: channel.Direction,
			}); updateErr != nil {
				return contracts.Output{}, updateErr
			}
			return contracts.Output{Data: map[string]any{"mailboxId": payload.MailboxID, "direction": payload.Direction, "status": "failed"},
				Events: []contracts.Event{mailEvent(ConnectionTestedEvent, "mail_channel", channel.ID, map[string]any{
					"mailboxId": payload.MailboxID, "direction": payload.Direction, "status": "failed",
				})}}, nil
		}
		if _, err = q.MailChannelVerificationUpdate(ctx, dbgen.MailChannelVerificationUpdateParams{
			VerificationStatus: "ok", LastErrorCode: "", TenantID: p.TenantID, MailboxID: payload.MailboxID, Direction: channel.Direction,
		}); err != nil {
			return contracts.Output{}, err
		}
		allChannels, err := q.MailChannelsList(ctx, dbgen.MailChannelsListParams{
			TenantID: p.TenantID, MailboxID: payload.MailboxID, OwnerAccountID: p.Account.ID,
		})
		if err != nil {
			return contracts.Output{}, problem(err)
		}
		verified := len(allChannels) == 2
		for _, item := range allChannels {
			verified = verified && item.VerificationStatus == "ok"
		}
		if verified {
			if _, err = q.MailMailboxStatusUpdate(ctx, dbgen.MailMailboxStatusUpdateParams{
				Status: "active", TenantID: p.TenantID, ID: payload.MailboxID, OwnerAccountID: p.Account.ID,
				Column5: []string{"unverified"},
			}); err != nil {
				return contracts.Output{}, problem(err)
			}
		}
		return contracts.Output{Data: map[string]any{"mailboxId": payload.MailboxID, "direction": payload.Direction, "status": "ok"},
			Events: []contracts.Event{mailEvent(ConnectionTestedEvent, "mail_channel", channel.ID, map[string]any{
				"mailboxId": payload.MailboxID, "direction": payload.Direction, "status": "ok",
			})}}, nil
	}
	return contracts.Output{}, contracts.Problem("not_found")
}
