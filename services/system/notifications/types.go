// Package notifications holds the domain types and the capability definitions of the
// notifications module. Types are owned by the domain, not by the transport layer.
package notifications

type AttachmentPayload struct {
	ID     int64 `json:"id"`
	FileID int64 `json:"fileID"`
}
type AudiencePayload struct {
}
type DeliveriesPayload struct {
}
type DeliveryRecord struct {
	Title         string `json:"title"`
	RecipientName string `json:"recipientName"`
	ID            int64  `json:"id"`
	MessageID     int64  `json:"messageId"`
	Channel       string `json:"channel"`
	Status        string `json:"status"`
	Attempts      int    `json:"attempts"`
	ErrorCode     string `json:"errorCode"`
	CreatedAt     string `json:"createdAt"`
	NextAttemptAt string `json:"nextAttemptAt"`
}
type FileContent struct {
	Name    string `json:"name"`
	Mime    string `json:"mime"`
	Content []byte `json:"content"`
}
type InboxMessage struct {
	Title          string                    `json:"title"`
	Markdown       string                    `json:"markdown"`
	Attachments    []*NotificationAttachment `json:"attachments"`
	ID             int64                     `json:"id"`
	Event          string                    `json:"event"`
	RequestID      int64                     `json:"requestId"`
	ApplicantName  string                    `json:"applicantName"`
	DepartmentName string                    `json:"departmentName"`
	Status         string                    `json:"status"`
	Read           bool                      `json:"read"`
	CreatedAt      string                    `json:"createdAt"`
}
type InboxPayload struct {
}
type MessagePayload struct {
	ID int64 `json:"id"`
}
type NotificationAttachment struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Mime string `json:"mime"`
}
type NotificationAudience struct {
	Name       string `json:"name"`
	Recipients int    `json:"recipients"`
	Kind       string `json:"kind"`
	ID         int64  `json:"id"`
}
type NotificationTarget struct {
	Kind string `json:"kind"`
	ID   int64  `json:"id"`
}
type PreviewPayload struct {
	Targets []*NotificationTarget `json:"targets"`
}
type PreviewResult struct {
	RecipientCount int `json:"recipientCount"`
}
type ReadPayload struct {
	ID int64 `json:"id"`
}
type RetryPayload struct {
	ID int64 `json:"id"`
}
type SendPayload struct {
	Targets   []*NotificationTarget `json:"targets"`
	ClientKey string                `json:"clientKey"`
	Title     string                `json:"title"`
	Markdown  string                `json:"markdown"`
	FileIds   []int64               `json:"fileIds"`
	Email     bool                  `json:"email"`
}
type SentNotification struct {
	Attachments    []*NotificationAttachment `json:"attachments"`
	ID             int64                     `json:"id"`
	Title          string                    `json:"title"`
	Markdown       string                    `json:"markdown"`
	RecipientCount int                       `json:"recipientCount"`
	CreatedAt      string                    `json:"createdAt"`
}
type SentPayload struct {
}
type TestPayload struct {
	Send bool `json:"send"`
}
type TestResult struct {
	Status string `json:"status"`
}
