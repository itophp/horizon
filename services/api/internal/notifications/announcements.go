package notifications

import (
	gen "baseline/services/api/gen/notifications"
	"baseline/services/api/internal/files"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func targetKey(t *gen.NotificationTarget) string { return t.Kind + ":" + strconv.FormatInt(t.ID, 10) }
func (s *Service) audience(ctx context.Context, q *dbgen.Queries, p *identity.Principal, scope identity.Scope) ([]dbgen.GovNotificationAudienceRow, error) {
	return q.GovNotificationAudience(ctx, dbgen.GovNotificationAudienceParams{TenantID: p.TenantID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
}
func (s *Service) Audience(ctx context.Context, _ *gen.AudiencePayload) (res []*gen.NotificationAudience, err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "notifications:send", true)
	if err != nil {
		return nil, err
	}
	rows, err := s.audience(ctx, s.Q, p, scope)
	if err != nil {
		return nil, err
	}
	res = []*gen.NotificationAudience{}
	for _, r := range rows {
		res = append(res, &gen.NotificationAudience{Kind: r.Kind, ID: r.TargetID, Name: r.Name, Recipients: int(r.Recipients)})
	}
	return res, nil
}
func (s *Service) recipients(ctx context.Context, q *dbgen.Queries, p *identity.Principal, scope identity.Scope, targets []*gen.NotificationTarget) ([]dbgen.GovAnnouncementRecipientsRow, error) {
	available, err := s.audience(ctx, q, p, scope)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, r := range available {
		allowed[r.Kind+":"+strconv.FormatInt(r.TargetID, 10)] = true
	}
	keys := []string{}
	for _, t := range targets {
		key := targetKey(t)
		if !allowed[key] {
			return nil, governance.Problem("forbidden")
		}
		keys = append(keys, key)
	}
	rows, err := q.GovAnnouncementRecipients(ctx, dbgen.GovAnnouncementRecipientsParams{TenantID: p.TenantID, TargetKeys: keys, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || len(rows) > 10000 {
		return nil, governance.Problem("invalid")
	}
	return rows, nil
}
func (s *Service) Preview(ctx context.Context, payload *gen.PreviewPayload) (res *gen.PreviewResult, err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "notifications:send", true)
	if err != nil {
		return nil, err
	}
	rows, err := s.recipients(ctx, s.Q, p, scope, payload.Targets)
	if err != nil {
		return nil, err
	}
	return &gen.PreviewResult{RecipientCount: len(rows)}, nil
}

var fileReference = regexp.MustCompile(`file:([0-9]+)`)

func validateContent(title, markdown string, ids []int64) error {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(markdown) == "" || strings.ContainsAny(title, "\r\n\x00") {
		return governance.Problem("invalid")
	}
	allowed := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || allowed[id] {
			return governance.Problem("invalid")
		}
		allowed[id] = true
	}
	for _, match := range fileReference.FindAllStringSubmatch(markdown, -1) {
		id, e := strconv.ParseInt(match[1], 10, 64)
		if e != nil || !allowed[id] {
			return governance.Problem("invalid")
		}
	}
	return nil
}
func (s *Service) Send(ctx context.Context, payload *gen.SendPayload) (res *gen.SentNotification, err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "notifications:send", true)
	if err != nil {
		return nil, err
	}
	if err = validateContent(payload.Title, payload.Markdown, payload.FileIds); err != nil {
		return nil, err
	}
	targets, err := json.Marshal(payload.Targets)
	if err != nil {
		return nil, err
	}
	// Cookie is never part of persisted content or the idempotency fingerprint.
	fingerprint, err := json.Marshal(struct {
		Title, Markdown string
		Targets         json.RawMessage
		Files           []int64
		Email           bool
	}{payload.Title, payload.Markdown, targets, payload.FileIds, payload.Email})
	if err != nil {
		return nil, err
	}
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		rows, e := s.recipients(ctx, q, p, scope, payload.Targets)
		if e != nil {
			return e
		}
		created, e := q.GovCreateAnnouncement(ctx, dbgen.GovCreateAnnouncementParams{TenantID: p.TenantID, SenderID: p.Account.ID, DepartmentID: p.Account.DepartmentID, ClientKey: payload.ClientKey, PayloadHash: governance.Hash(string(fingerprint)), Title: payload.Title, Markdown: payload.Markdown, Targets: targets, Email: payload.Email, RecipientCount: int32(len(rows))})
		if e != nil {
			return e
		}
		if created.PayloadHash != governance.Hash(string(fingerprint)) || created.DepartmentID != p.Account.DepartmentID {
			return governance.Problem("conflict")
		}
		res = &gen.SentNotification{ID: created.ID, Title: created.Title, Markdown: created.Markdown, RecipientCount: int(created.RecipientCount), CreatedAt: governance.Time(created.CreatedAt)}
		if !created.Inserted {
			return nil
		}
		attachments := []*gen.NotificationAttachment{}
		fileIDs := append([]int64{}, payload.FileIds...)
		sort.Slice(fileIDs, func(i, j int) bool { return fileIDs[i] < fileIDs[j] })
		for _, id := range fileIDs {
			if e = q.GovLockAttachment(ctx, dbgen.GovLockAttachmentParams{TenantID: p.TenantID, FileID: id}); e != nil {
				return e
			}
			file, e := q.GovFile(ctx, dbgen.GovFileParams{TenantID: p.TenantID, ID: id, OwnID: p.Account.ID})
			if e != nil {
				return e
			}
			if file.Purpose != "attachment" {
				return governance.Problem("invalid")
			}
			if _, e = q.GovBindAnnouncementFile(ctx, dbgen.GovBindAnnouncementFileParams{TenantID: p.TenantID, AnnouncementID: created.ID, ID: id, OwnerID: p.Account.ID}); e != nil {
				return e
			}
			attachments = append(attachments, &gen.NotificationAttachment{ID: id, Name: file.Name, Mime: file.Mime})
		}
		for _, r := range rows {
			values, e := s.ValuesWith(ctx, q, p.TenantID, []string{"workspace:0", governance.TargetKey("account", r.AccountID)})
			if e != nil {
				return e
			}
			inApp, e := governance.Bool(values["notifications.inApp"].Raw)
			if e != nil {
				return e
			}
			data := eventPayload{Event: "announcement", Title: payload.Title, Markdown: payload.Markdown, Attachments: attachments, InApp: inApp}
			raw, e := json.Marshal(data)
			if e != nil {
				return e
			}
			id, e := q.GovAddAnnouncementMessage(ctx, dbgen.GovAddAnnouncementMessageParams{TenantID: p.TenantID, AnnouncementID: governance.Int(created.ID), RecipientID: r.AccountID, DepartmentID: r.DepartmentID, Payload: raw})
			if e != nil {
				return e
			}
			status, code := "sent", ""
			if !inApp {
				status, code = "skipped", "preference_disabled"
			}
			if e = q.GovAddInAppDelivery(ctx, dbgen.GovAddInAppDeliveryParams{TenantID: p.TenantID, MessageID: id, Status: status, ErrorCode: code}); e != nil {
				return e
			}
			if payload.Email {
				if e = q.GovAddDelivery(ctx, dbgen.GovAddDeliveryParams{TenantID: p.TenantID, MessageID: id, Status: "pending"}); e != nil {
					return e
				}
			}
		}
		t := governance.Target{Kind: "department", ID: p.Account.DepartmentID, Key: governance.TargetKey("department", p.Account.DepartmentID)}
		// No Markdown or recipient addresses in audit.
		return s.Audit(ctx, q, p, t, "notifications.send", fmt.Sprintf("announcement:%d", created.ID), []byte(fmt.Sprintf(`{"recipients":%d}`, created.RecipientCount)))
	})
	if err != nil {
		return nil, err
	}
	res.Attachments, err = s.announcementFiles(ctx, p.TenantID, res.ID)
	return res, err
}
func (s *Service) Sent(ctx context.Context, _ *gen.SentPayload) (res []*gen.SentNotification, err error) {
	defer apiError(&err)
	p, _, err := s.Identity.Authorize(ctx, "notifications:send", true)
	if err != nil {
		return nil, err
	}
	rows, err := s.Q.GovSentAnnouncements(ctx, dbgen.GovSentAnnouncementsParams{TenantID: p.TenantID, SenderID: p.Account.ID, DepartmentID: p.Account.DepartmentID})
	if err != nil {
		return nil, err
	}
	res = []*gen.SentNotification{}
	for _, r := range rows {
		attachments, e := s.announcementFiles(ctx, p.TenantID, r.ID)
		if e != nil {
			return nil, e
		}
		res = append(res, &gen.SentNotification{ID: r.ID, Title: r.Title, Markdown: r.Markdown, Attachments: attachments, RecipientCount: int(r.RecipientCount), CreatedAt: governance.Time(r.CreatedAt)})
	}
	return res, nil
}
func (s *Service) Attachment(ctx context.Context, payload *gen.AttachmentPayload) (res *gen.FileContent, err error) {
	defer apiError(&err)
	f, ok := s.Logos.(*files.Service)
	if !ok {
		return nil, governance.Problem("unavailable")
	}
	name, mime, data, err := f.NotificationContent(ctx, payload.ID, payload.FileID)
	if err != nil {
		return nil, err
	}
	return &gen.FileContent{Name: name, Mime: mime, Content: data}, nil
}

func (s *Service) announcementFiles(ctx context.Context, tenant string, id int64) ([]*gen.NotificationAttachment, error) {
	rows, err := s.Q.GovAnnouncementFiles(ctx, dbgen.GovAnnouncementFilesParams{TenantID: tenant, AnnouncementID: id})
	if err != nil {
		return nil, err
	}
	out := []*gen.NotificationAttachment{}
	for _, r := range rows {
		out = append(out, &gen.NotificationAttachment{ID: r.ID, Name: r.Name, Mime: r.Mime})
	}
	return out, nil
}

func (s *Service) Message(ctx context.Context, payload *gen.MessagePayload) (res *gen.InboxMessage, err error) {
	defer apiError(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	row, err := s.Q.GovInboxMessage(ctx, dbgen.GovInboxMessageParams{TenantID: p.TenantID, RecipientID: p.Account.ID, ID: payload.ID, DepartmentID: p.Account.DepartmentID})
	if err != nil {
		return nil, err
	}
	var data eventPayload
	if err = json.Unmarshal(row.Payload, &data); err != nil {
		return nil, err
	}
	if data.Event == "requested" && !p.Can("temporary:approve") {
		return nil, governance.Problem("not_found")
	}
	if data.Attachments == nil {
		data.Attachments = []*gen.NotificationAttachment{}
	}
	return &gen.InboxMessage{ID: row.ID, Title: data.Title, Markdown: data.Markdown, Attachments: data.Attachments, Event: data.Event, RequestID: data.RequestID, ApplicantName: data.ApplicantName, DepartmentName: data.DepartmentName, Status: data.Status, Read: row.ReadAt.Valid, CreatedAt: governance.Time(row.CreatedAt)}, nil
}
