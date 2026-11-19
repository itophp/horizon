package mail

import (
	"sync"

	"github.com/knadh/go-pop3"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/governance"
)

// Module owns personal external-mail capabilities. The governance core supplies
// the trusted principal, transaction helper and credential root key; the
// business limits are plugin options resolved by the deployment.
type Module struct {
	*governance.Core
	options  Options
	sessions sync.Map
}

func New(core *governance.Core, options Options) *Module {
	if core == nil {
		core = governance.New(nil, nil, governance.Options{})
	}
	return &Module{Core: core, options: options.defaults()}
}

func (m *Module) session(id string) (*pop3.Conn, bool) {
	value, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	connection, ok := value.(*pop3.Conn)
	return connection, ok
}

func (m *Module) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: mailboxListDefinition(), Handler: contracts.Handler{Execute: m.listMailboxes}},
		{Definition: mailboxGetDefinition(), Handler: contracts.Handler{Execute: m.getMailbox}},
		{Definition: mailboxOnboardDefinition(), Handler: contracts.Handler{ExecuteTx: m.onboard}},
		{Definition: mailboxUpdateDefinition(), Handler: contracts.Handler{ExecuteTx: m.updateMailbox}},
		{Definition: credentialReplaceDefinition(), Handler: contracts.Handler{ExecuteTx: m.replaceCredential}},
		{Definition: mailboxEnableDefinition(), Handler: contracts.Handler{ExecuteTx: m.enableMailbox}},
		{Definition: mailboxDisableDefinition(), Handler: contracts.Handler{ExecuteTx: m.disableMailbox}},
		{Definition: connectionTestDefinition(), Handler: contracts.Handler{ExecuteTx: m.testConnection}},
		{Definition: messageListDefinition(), Handler: contracts.Handler{Execute: m.listMessages}},
		{Definition: messageGetDefinition(), Handler: contracts.Handler{Execute: m.getMessage}},
		{Definition: messageFlagDefinition(), Handler: contracts.Handler{ExecuteTx: m.updateMessageFlag}},
		{Definition: messageSendDefinition(), Handler: contracts.Handler{ExecuteTx: m.sendMessage}},
		{Definition: attachmentReadDefinition(), Handler: contracts.Handler{Execute: m.readAttachment}},
		{Definition: deliveryListDefinition(), Handler: contracts.Handler{Execute: m.listDeliveries}},
		{Definition: deliveryRetryDefinition(), Handler: contracts.Handler{ExecuteTx: m.retryDelivery}},
		{Definition: deliveryCancelDefinition(), Handler: contracts.Handler{ExecuteTx: m.cancelDelivery}},
		{Definition: outboundProcessDefinition(), Handler: contracts.Handler{ExecuteTx: m.processOutbound}},
		{Definition: inboxSyncDefinition(), Handler: contracts.Handler{ExecuteTx: m.syncInbox}},
	}
}
