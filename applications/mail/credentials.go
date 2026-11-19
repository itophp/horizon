package mail

import (
	"fmt"

	"horizon/foundation/system/governance"
)

func credentialScope(mailboxID int64) string {
	return fmt.Sprintf("mail_mailbox:%d", mailboxID)
}

func credentialKey(purpose string, version int32) string {
	return fmt.Sprintf("%s:v%d", purpose, version)
}

func sealCredential(core *governance.Core, tenant string, mailboxID int64, purpose, password string) (string, string, error) {
	ciphertext, err := core.Seal(tenant, credentialScope(mailboxID), credentialKey(purpose, 1), password)
	if err != nil {
		return "", "", err
	}
	return ciphertext, governance.Hash(ciphertext)[:16], nil
}

func openCredential(core *governance.Core, tenant string, mailboxID int64, purpose string, version int32, ciphertext string) (string, error) {
	return core.Unseal(tenant, credentialScope(mailboxID), credentialKey(purpose, version), ciphertext)
}
