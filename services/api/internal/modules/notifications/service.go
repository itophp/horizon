package notifications

import (
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type Service struct{ *governance.Core }

func New(c *governance.Core) *Service { return &Service{c} }
func apiError(err *error) {
	if *err != nil {
		*err = governance.Problem(governance.Code(*err))
	}
}

type eventPayload struct {
	Title          string                    `json:"title"`
	Markdown       string                    `json:"markdown"`
	Attachments    []*NotificationAttachment `json:"attachments"`
	Event          string                    `json:"event"`
	RequestID      int64                     `json:"requestId"`
	ApplicantName  string                    `json:"applicantName"`
	DepartmentName string                    `json:"departmentName"`
	Status         string                    `json:"status"`
	InApp          bool                      `json:"inApp"`
}

func (s *Service) enabled(ctx context.Context, p *identity.Principal, key string) (bool, error) {
	raw, e := s.Setting(ctx, p, key)
	if e != nil {
		return false, e
	}
	return governance.Bool(raw)
}
func (s *Service) Inbox(ctx context.Context, _ *InboxPayload) (res []*InboxMessage, err error) {
	defer apiError(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.Q.GovInbox(ctx, dbgen.GovInboxParams{TenantID: p.TenantID, RecipientID: p.Account.ID, DepartmentID: p.Account.DepartmentID})
	if err != nil {
		return nil, err
	}
	return s.inboxMessages(rows, p)
}
func (s *Service) InboxPage(ctx context.Context, offset, limit int) ([]*InboxMessage, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	if offset < 0 || offset > 10000 || limit < 1 || limit > 50 {
		return nil, governance.Problem("invalid")
	}
	rows, err := s.Q.GovInboxPage(ctx, dbgen.GovInboxPageParams{TenantID: p.TenantID, RecipientID: p.Account.ID, DepartmentID: p.Account.DepartmentID, CanReview: p.Can("temporary:approve"), PageLimit: int32(limit), PageOffset: int32(offset)})
	if err != nil {
		return nil, err
	}
	converted := make([]dbgen.GovInboxRow, len(rows))
	for i, row := range rows {
		converted[i] = dbgen.GovInboxRow(row)
	}
	return s.inboxMessages(converted, p)
}
func (s *Service) inboxMessages(rows []dbgen.GovInboxRow, p *identity.Principal) (res []*InboxMessage, err error) {
	res = []*InboxMessage{}
	for _, r := range rows {
		var data eventPayload
		if err = json.Unmarshal(r.Payload, &data); err != nil {
			return nil, err
		}
		if !data.InApp {
			continue
		}
		if data.Event == "requested" && !p.Can("temporary:approve") {
			continue
		}
		if data.Attachments == nil {
			data.Attachments = []*NotificationAttachment{}
		}
		res = append(res, &InboxMessage{Title: data.Title, Markdown: data.Markdown, Attachments: data.Attachments, ID: r.ID, Event: data.Event, RequestID: data.RequestID, ApplicantName: data.ApplicantName, DepartmentName: data.DepartmentName, Status: data.Status, Read: r.ReadAt.Valid, CreatedAt: governance.Time(r.CreatedAt)})
	}
	return res, nil
}
func (s *Service) Read(ctx context.Context, payload *ReadPayload) (err error) {
	defer apiError(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return err
	}
	_, err = s.Q.GovReadNotification(ctx, dbgen.GovReadNotificationParams{TenantID: p.TenantID, ID: payload.ID, RecipientID: p.Account.ID, DepartmentID: p.Account.DepartmentID})
	return err
}
func (s *Service) Deliveries(ctx context.Context, _ *DeliveriesPayload) (res []*DeliveryRecord, err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "notifications:read", true)
	if err != nil {
		return nil, err
	}
	rows, err := s.Q.GovDeliveries(ctx, dbgen.GovDeliveriesParams{TenantID: p.TenantID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	if err != nil {
		return nil, err
	}
	res = []*DeliveryRecord{}
	for _, r := range rows {
		res = append(res, &DeliveryRecord{Title: r.Title, RecipientName: r.RecipientName, ID: r.ID, MessageID: r.MessageID, Channel: r.Channel, Status: r.Status, Attempts: int(r.Attempts), ErrorCode: r.ErrorCode, CreatedAt: governance.Time(r.CreatedAt), NextAttemptAt: governance.Time(r.NextAttemptAt)})
	}
	return res, nil
}
func (s *Service) Retry(ctx context.Context, payload *RetryPayload) (err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "notifications:retry", true)
	if err != nil {
		return err
	}
	return s.Transaction(ctx, func(q *dbgen.Queries) error {
		if _, e := q.GovRetryDelivery(ctx, dbgen.GovRetryDeliveryParams{TenantID: p.TenantID, ID: payload.ID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs}); e != nil {
			return e
		}
		t := governance.Target{Kind: "department", ID: p.Account.DepartmentID, Key: governance.TargetKey("department", p.Account.DepartmentID)}
		return s.Audit(ctx, q, p, t, "notifications.retry", "delivery:"+strconv.FormatInt(payload.ID, 10), []byte(`{}`))
	})
}
func (s *Service) Test(ctx context.Context, payload *TestPayload) (res *TestResult, err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "notifications:test", true)
	if err != nil {
		return nil, err
	}
	t, err := s.Resolve(ctx, p.TenantID, "company", p.Account.OrganizationID)
	if err != nil {
		return nil, err
	}
	if !governance.Allowed(scope, t) {
		return nil, governance.Problem("forbidden")
	}
	config, err := s.smtpConfiguration(ctx, p)
	if err != nil {
		return nil, err
	}
	email, err := s.Q.GovAccountEmail(ctx, dbgen.GovAccountEmailParams{TenantID: p.TenantID, ID: p.Account.ID})
	if err != nil {
		return nil, err
	}
	if !payload.Send && email == "" {
		email = config.From
	}
	detail, e := json.Marshal(map[string]bool{"send": payload.Send})
	if e != nil {
		return nil, e
	}
	if e = s.Transaction(ctx, func(q *dbgen.Queries) error { return s.Audit(ctx, q, p, t, "notifications.test", "smtp", detail) }); e != nil {
		return nil, e
	}
	if err = s.send(ctx, config, email, "SMTP connection test", "SMTP connection test", governance.Hash(p.TokenHash + time.Now().String())[:32], payload.Send); err != nil {
		return nil, governance.Problem("unavailable")
	}
	return &TestResult{Status: "sent"}, nil
}

// ProcessOnce handles one event and one delivery. Event fanout and completion share
// a transaction; unique keys make lease recovery idempotent across worker crashes.
func (s *Service) ProcessOnce(ctx context.Context) error {
	if err := s.Available(); err != nil {
		return err
	}
	if err := s.Q.GovExpireDeliveryLeases(ctx); err != nil {
		return err
	}
	event, err := s.Q.GovClaimEvent(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		if err = s.fanout(ctx, event); err != nil {
			return err
		}
	}
	delivery, err := s.Q.GovClaimDelivery(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.deliver(ctx, delivery)
}
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.ProcessOnce(ctx); err != nil {
				slog.Error("notification worker failed", "code", governance.Code(err))
			}
		}
	}
}
func (s *Service) fanout(ctx context.Context, event dbgen.GovClaimEventRow) error {
	r, err := s.Q.GovEventRequest(ctx, dbgen.GovEventRequestParams{TenantID: event.TenantID, ID: event.RequestID})
	if err != nil {
		return err
	}
	recipients := []*identity.Principal{}
	if event.Event == "requested" {
		candidates, err := s.Q.GovCandidateMemberships(ctx, event.TenantID)
		if err != nil {
			return err
		}
		for _, candidate := range candidates {
			if candidate.AccountID == r.ApplicantID {
				continue
			}
			p, e := s.Identity.PrincipalForTask(ctx, event.TenantID, candidate.AccountID, candidate.DepartmentID)
			if e != nil {
				if governance.Code(e) == "forbidden" {
					continue
				}
				return e
			}
			if p.Can("temporary:approve") {
				recipients = append(recipients, p)
			}
		}
	} else if !(event.Event == "revoked" && r.RevokedBy.Int64 == r.ApplicantID) {
		p, e := s.Identity.PrincipalForTask(ctx, event.TenantID, r.ApplicantID, r.DepartmentID)
		if e == nil {
			recipients = append(recipients, p)
		} else if governance.Code(e) != "forbidden" && governance.Code(e) != "not_found" {
			return e
		}
	}
	preferences := map[*identity.Principal]bool{}
	for _, p := range recipients {
		enabled, e := s.enabled(ctx, p, "notifications.inApp")
		if e != nil {
			return e
		}
		preferences[p] = enabled
	}
	return s.Transaction(ctx, func(q *dbgen.Queries) error {
		for _, p := range recipients {
			inApp := preferences[p]
			data := eventPayload{Event: event.Event, RequestID: event.RequestID, ApplicantName: r.FullName, DepartmentName: r.DepartmentName, Status: event.Event, InApp: inApp}
			raw, e := json.Marshal(data)
			if e != nil {
				return e
			}
			id, e := q.GovAddNotification(ctx, dbgen.GovAddNotificationParams{TenantID: event.TenantID, EventID: governance.Int(event.ID), RecipientID: p.Account.ID, DepartmentID: p.Account.DepartmentID, Payload: raw})
			if e != nil {
				return e
			}
			if e = q.GovAddDelivery(ctx, dbgen.GovAddDeliveryParams{TenantID: event.TenantID, MessageID: id, Status: "pending"}); e != nil {
				return e
			}
		}
		return q.GovEventDone(ctx, dbgen.GovEventDoneParams{TenantID: event.TenantID, ID: event.ID})
	})
}
func (s *Service) deliver(ctx context.Context, d dbgen.GovClaimDeliveryRow) error {
	finish := func(status, code string) error {
		return s.Q.GovFinishDelivery(ctx, dbgen.GovFinishDeliveryParams{TenantID: d.TenantID, ID: d.ID, Status: status, ErrorCode: code, Column5: int32(30 * (1 << uint(d.Attempts-1)))})
	}
	r, err := s.Q.GovDeliveryMessage(ctx, dbgen.GovDeliveryMessageParams{TenantID: d.TenantID, ID: d.MessageID})
	if err != nil {
		return err
	}
	p, err := s.Identity.PrincipalForTask(ctx, d.TenantID, r.RecipientID, r.DepartmentID)
	if err != nil {
		code := governance.Code(err)
		if code == "forbidden" || code == "not_found" {
			return finish("skipped", "recipient_unavailable")
		}
		return err
	}
	var data eventPayload
	if err = json.Unmarshal(r.Payload, &data); err != nil {
		return err
	}
	if data.Event == "requested" && !p.Can("temporary:approve") {
		return finish("skipped", "permission_revoked")
	}
	enabled, err := s.enabled(ctx, p, "notifications.email")
	if err != nil {
		return err
	}
	if !enabled {
		return finish("skipped", "preference_disabled")
	}
	if r.Email == "" {
		return finish("skipped", "email_missing")
	}
	config, err := s.smtpConfiguration(ctx, p)
	if err != nil {
		return err
	}
	if !config.Enabled {
		return finish("skipped", "smtp_disabled")
	}
	language, err := s.Setting(ctx, p, "display.language")
	if err != nil {
		return err
	}
	locale, err := governance.String(language)
	if err != nil {
		return err
	}
	if locale != "zh-CN" {
		locale = "en"
	}
	render := func(kind string) (string, error) {
		if data.Event == "announcement" {
			if kind == "subject" {
				return data.Title, nil
			}
			link := strings.TrimRight(s.Options.PublicURL, "/") + "/notifications?message=" + strconv.FormatInt(d.MessageID, 10)
			return fileReference.ReplaceAllString(data.Markdown, link) + "\n\n" + link, nil
		}
		raw, e := s.Setting(ctx, p, "templates."+data.Event+"."+locale+"."+kind)
		if e != nil {
			return "", e
		}
		text, e := governance.String(raw)
		if e != nil {
			return "", e
		}
		return strings.NewReplacer("{{applicant}}", data.ApplicantName, "{{department}}", data.DepartmentName, "{{requestId}}", strconv.FormatInt(data.RequestID, 10), "{{status}}", data.Status, "{{link}}", strings.TrimRight(s.Options.PublicURL, "/")+"/temporary-access").Replace(text), nil
	}
	subject, err := render("subject")
	if err != nil {
		return err
	}
	body, err := render("body")
	if err != nil {
		return err
	}
	timeout, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err = s.send(timeout, config, r.Email, subject, body, governance.Hash(d.TenantID + ":" + strconv.FormatInt(d.ID, 10))[:32], true); err != nil {
		status := "pending"
		if d.Attempts >= 5 {
			status = "failed"
		}
		return finish(status, "smtp_failed")
	}
	return finish("sent", "")
}
