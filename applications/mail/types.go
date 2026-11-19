package mail

type OnboardInput struct {
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	Password     string `json:"password"`
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPTLS      string `json:"smtpTls"`
	SMTPUsername string `json:"smtpUsername"`
	POP3Host     string `json:"pop3Host"`
	POP3Port     int    `json:"pop3Port"`
	POP3Username string `json:"pop3Username"`
}

type MailboxUpdateInput struct {
	ID           int64  `json:"id"`
	Revision     int64  `json:"revision"`
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	DefaultSend  bool   `json:"defaultSend"`
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPTLS      string `json:"smtpTls"`
	SMTPUsername string `json:"smtpUsername"`
	POP3Host     string `json:"pop3Host"`
	POP3Port     int    `json:"pop3Port"`
	POP3Username string `json:"pop3Username"`
}

type CredentialInput struct {
	MailboxID int64  `json:"mailboxId"`
	Purpose   string `json:"purpose"`
	Password  string `json:"password"`
}

type MailboxStatusInput struct {
	ID int64 `json:"id"`
}

type ConnectionTestInput struct {
	MailboxID int64  `json:"mailboxId"`
	Direction string `json:"direction"`
}

type MessageListInput struct {
	MailboxID int64  `json:"mailboxId"`
	Folder    string `json:"folder"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

type MessageGetInput struct {
	ID int64 `json:"id"`
}

type MessageFlagInput struct {
	ID   int64 `json:"id"`
	Read bool  `json:"read"`
	Star bool  `json:"star"`
}

type Address struct {
	Address     string `json:"address"`
	DisplayName string `json:"displayName,omitempty"`
}

type SendMessage struct {
	ClientKey string    `json:"clientKey"`
	MailboxID int64     `json:"mailboxId"`
	To        []Address `json:"to"`
	CC        []Address `json:"cc"`
	BCC       []Address `json:"bcc"`
	Subject   string    `json:"subject"`
	Markdown  string    `json:"markdown"`
	FileIDs   []int64   `json:"fileIds"`
	ReplyTo   string    `json:"replyTo"`
}

type DeliveryAction struct {
	ID int64 `json:"id"`
}
