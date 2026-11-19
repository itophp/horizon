package mail

import (
	"testing"

	"horizon/foundation/system/governance"
)

func TestCredentialEncryptionBindsMailboxPurposeAndVersion(t *testing.T) {
	core := governance.New(nil, nil, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	ciphertext, _, err := sealCredential(core, "tenant", 7, "smtp", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if opened, err := openCredential(core, "tenant", 7, "smtp", 1, ciphertext); err != nil || opened != "secret" {
		t.Fatalf("credential did not round trip: %q %v", opened, err)
	}
	for _, args := range [][4]any{{"other", 7, "smtp", 1}, {"tenant", 8, "smtp", 1}, {"tenant", 7, "pop3", 1}, {"tenant", 7, "smtp", 2}} {
		if opened, err := openCredential(core, args[0].(string), int64(args[1].(int)), args[2].(string), int32(args[3].(int)), ciphertext); err == nil {
			t.Fatalf("ciphertext opened with wrong AAD: %q", opened)
		}
	}
}
