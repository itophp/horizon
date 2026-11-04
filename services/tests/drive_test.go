//go:build integration

package tests

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"baseline/services/api/applications/drive"
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/approval"
	"baseline/services/api/kernel/audit"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/eventhub"
	"baseline/services/api/system/files"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
	"github.com/jackc/pgx/v5"
)

func newDriveHarness(t *testing.T) *projectHarness {
	t.Helper()
	h := newProjectHarness(t)
	core := governance.New(h.pool, h.iam, governance.Options{StorageRoot: t.TempDir()})
	catalog := registry.New(drive.NewWithFiles(h.pool, files.New(core)).Entries())
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	h.gw = gateway.New(gateway.Options{Registry: catalog, Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	return h
}

func TestDriveUploadChunksAreBoundedAndPrivate(t *testing.T) {
	h := newDriveHarness(t)
	owner := h.account(t, "drive-upload-owner", "drive:use")
	other := h.account(t, "drive-upload-other", "drive:use")
	space := rowID(t, h.ok(t, owner, "drive.space.personal.ensure", nil))
	upload := rowID(t, h.ok(t, owner, "drive.upload.start", map[string]any{"spaceId": space, "name": "data.bin", "size": 4}))
	usage := decodePayload[struct {
		Used    int64 `json:"usedBytes"`
		Pending int64 `json:"pendingBytes"`
	}](t, h.ok(t, owner, "drive.space.usage", map[string]any{"spaceId": space}).Data)
	if usage.Used != 0 || usage.Pending != 4 {
		t.Fatalf("unexpected reserved capacity: %+v", usage)
	}
	_, err := h.call(t, other, "drive.space.usage", map[string]any{"spaceId": space})
	if contracts.Code(err) != "not_found" {
		t.Fatalf("other user accessed capacity: %v", err)
	}
	data := []byte("data")
	digest := sha256.Sum256(data)
	chunk := map[string]any{"spaceId": space, "uploadId": upload, "index": 0, "sha256": hex.EncodeToString(digest[:]), "content": base64.StdEncoding.EncodeToString(data)}
	h.ok(t, owner, "drive.upload.chunk", chunk)
	h.ok(t, owner, "drive.upload.chunk", chunk)
	status := decodePayload[struct {
		Completed []struct {
			Index  int    `json:"index"`
			SHA256 string `json:"sha256"`
		} `json:"completedChunks"`
	}](t, h.ok(t, owner, "drive.upload.status", map[string]any{"id": upload}).Data)
	if len(status.Completed) != 1 || status.Completed[0].Index != 0 || status.Completed[0].SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("unexpected uploaded chunks: %+v", status.Completed)
	}
	_, err = h.call(t, other, "drive.upload.status", map[string]any{"id": upload})
	if contracts.Code(err) != "not_found" {
		t.Fatalf("other user accessed upload: %v", err)
	}
	bad := map[string]any{"spaceId": space, "uploadId": upload, "index": 0, "sha256": hex.EncodeToString(digest[:]), "content": base64.StdEncoding.EncodeToString([]byte("xxxx"))}
	_, err = h.call(t, owner, "drive.upload.chunk", bad)
	if contracts.Code(err) != "invalid" {
		t.Fatalf("invalid digest accepted: %v", err)
	}
	driveFile := rowID(t, h.ok(t, owner, "drive.upload.complete", map[string]any{"spaceId": space, "uploadId": upload}))
	finished := decodePayload[struct {
		CompletedFileID int64 `json:"completedFileId"`
	}](t, h.ok(t, owner, "drive.upload.status", map[string]any{"id": upload}).Data)
	if finished.CompletedFileID != driveFile {
		t.Fatalf("completion recovery returned %d, want %d", finished.CompletedFileID, driveFile)
	}
	read := decodePayload[struct {
		Content string `json:"content"`
		EOF     bool   `json:"eof"`
	}](t,
		h.ok(t, owner, "drive.file.chunk.read", map[string]any{"spaceId": space, "fileId": driveFile, "offset": 0}).Data)
	if read.Content != base64.StdEncoding.EncodeToString(data) || !read.EOF {
		t.Fatalf("incorrect downloaded content: %+v", read)
	}
	_, err = h.call(t, other, "drive.file.chunk.read", map[string]any{"spaceId": space, "fileId": driveFile, "offset": 0})
	if contracts.Code(err) != "not_found" {
		t.Fatalf("other user accessed file: %v", err)
	}
	var fileID int64
	var storedHash string
	if err = h.pool.QueryRow(context.Background(), `SELECT f.id,f.sha256 FROM drive_files d JOIN gov_files f ON f.tenant_id=d.tenant_id AND f.id=d.file_id WHERE d.tenant_id='tenant-a' AND d.id=$1`, driveFile).Scan(&fileID, &storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash != hex.EncodeToString(digest[:]) {
		t.Fatalf("assembled hash = %s", storedHash)
	}
	_, err = dbgen.New(h.pool).GovFile(context.Background(), dbgen.GovFileParams{TenantID: "tenant-a", ID: fileID, AllData: true})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cloud file exposed through general files: %v", err)
	}
	folder := rowID(t, h.ok(t, owner, "drive.folder.create", map[string]any{"spaceId": space, "name": "Archive"}))
	h.ok(t, owner, "drive.file.rename", map[string]any{"spaceId": space, "fileId": driveFile, "name": "renamed.bin"})
	h.ok(t, owner, "drive.file.move", map[string]any{"spaceId": space, "fileId": driveFile, "folderId": folder})
	batch := rowID(t, h.ok(t, owner, "drive.folder.trash", map[string]any{"spaceId": space, "folderId": folder}))
	_, err = h.call(t, owner, "drive.file.chunk.read", map[string]any{"spaceId": space, "fileId": driveFile, "offset": 0})
	if contracts.Code(err) != "not_found" {
		t.Fatalf("trashed child file was readable: %v", err)
	}
	h.ok(t, owner, "drive.trash.restore", map[string]any{"spaceId": space, "batchId": batch, "parentId": 0, "name": "Archive"})
	h.ok(t, owner, "drive.file.chunk.read", map[string]any{"spaceId": space, "fileId": driveFile, "offset": 0})
	fileBatch := rowID(t, h.ok(t, owner, "drive.file.trash", map[string]any{"spaceId": space, "fileId": driveFile}))
	h.ok(t, owner, "drive.trash.file.restore", map[string]any{"spaceId": space, "batchId": fileBatch, "folderId": folder, "name": "renamed.bin"})
	h.ok(t, owner, "drive.file.chunk.read", map[string]any{"spaceId": space, "fileId": driveFile, "offset": 0})
	var logged string
	if err = h.pool.QueryRow(context.Background(), `SELECT input::text FROM cap_invocations WHERE tenant_id='tenant-a' AND capability='drive.upload.chunk' ORDER BY id LIMIT 1`).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logged, base64.StdEncoding.EncodeToString(data)) || strings.Contains(logged, hex.EncodeToString(digest[:])) {
		t.Fatalf("chunk content leaked to audit: %s", logged)
	}
	empty := rowID(t, h.ok(t, owner, "drive.upload.start", map[string]any{"spaceId": space, "name": "empty", "size": 0}))
	h.ok(t, owner, "drive.upload.complete", map[string]any{"spaceId": space, "uploadId": empty})
}

func TestDriveSpaceIsolationAndRevocation(t *testing.T) {
	h := newDriveHarness(t)
	owner := h.account(t, "drive-owner", "drive:use", "drive:create")
	reader := h.account(t, "drive-reader", "drive:use")
	outsider := h.account(t, "drive-outsider", "drive:use")
	personal := rowID(t, h.ok(t, owner, "drive.space.personal.ensure", nil))
	if again := rowID(t, h.ok(t, owner, "drive.space.personal.ensure", nil)); again != personal {
		t.Fatalf("personal space changed: %d != %d", again, personal)
	}
	shared := rowID(t, h.ok(t, owner, "drive.space.create", map[string]any{"name": "Team"}))
	_, err := h.call(t, reader, "drive.space.create", map[string]any{"name": "Denied"})
	if contracts.Code(err) != "forbidden" {
		t.Fatalf("shared space creation must require permission: %v", err)
	}
	h.ok(t, owner, "drive.member.add", map[string]any{"spaceId": shared, "targetType": "account", "targetId": reader.id, "role": "viewer"})
	folder := rowID(t, h.ok(t, owner, "drive.folder.create", map[string]any{"spaceId": shared, "name": "Documents"}))
	nested := rowID(t, h.ok(t, owner, "drive.folder.create", map[string]any{"spaceId": shared, "parentId": folder, "name": "Nested"}))
	_, err = h.call(t, owner, "drive.folder.move", map[string]any{"spaceId": shared, "id": folder, "parentId": nested})
	if contracts.Code(err) != "not_found" {
		t.Fatalf("moving into a descendant must fail: %v", err)
	}
	h.ok(t, owner, "drive.folder.rename", map[string]any{"spaceId": shared, "id": nested, "name": "Renamed"})
	h.ok(t, owner, "drive.folder.move", map[string]any{"spaceId": shared, "id": nested, "parentId": 0})
	_, err = h.call(t, owner, "drive.folder.create", map[string]any{"spaceId": shared, "name": "Documents"})
	if contracts.Code(err) != "conflict" {
		t.Fatalf("duplicate folder must conflict: %v", err)
	}
	_, err = h.call(t, reader, "drive.folder.create", map[string]any{"spaceId": shared, "name": "Forbidden"})
	if contracts.Code(err) != "not_found" {
		t.Fatalf("viewer must not create a folder: %v", err)
	}
	folders := decodePayload[struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}](t,
		h.ok(t, reader, "drive.folder.list", map[string]any{"spaceId": shared}).Data)
	if len(folders.Items) != 2 {
		t.Fatalf("unexpected root folders: %+v", folders.Items)
	}
	other := decodePayload[struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}](t,
		h.ok(t, outsider, "drive.folder.list", map[string]any{"spaceId": shared}).Data)
	if len(other.Items) != 0 {
		t.Fatalf("outsider can see folders: %+v", other.Items)
	}
	child := rowID(t, h.ok(t, owner, "drive.folder.create", map[string]any{"spaceId": shared, "parentId": folder, "name": "Child"}))
	batch := rowID(t, h.ok(t, owner, "drive.folder.trash", map[string]any{"spaceId": shared, "folderId": folder}))
	trash := decodePayload[struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}](t,
		h.ok(t, owner, "drive.trash.list", map[string]any{"spaceId": shared}).Data)
	if len(trash.Items) != 1 || trash.Items[0].ID != batch {
		t.Fatalf("unexpected trash: %+v", trash.Items)
	}
	h.ok(t, owner, "drive.folder.create", map[string]any{"spaceId": shared, "name": "Documents"})
	_, err = h.call(t, owner, "drive.trash.restore", map[string]any{"spaceId": shared, "batchId": batch, "parentId": 0, "name": "Documents"})
	if contracts.Code(err) != "conflict" {
		t.Fatalf("restore with conflicting name must fail: %v", err)
	}
	h.ok(t, owner, "drive.trash.restore", map[string]any{"spaceId": shared, "batchId": batch, "parentId": 0, "name": "Restored"})
	children := decodePayload[struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}](t,
		h.ok(t, owner, "drive.folder.list", map[string]any{"spaceId": shared, "parentId": folder}).Data)
	if len(children.Items) != 1 || children.Items[0].ID != child {
		t.Fatalf("folder subtree was not restored: %+v", children.Items)
	}
	_, err = h.call(t, owner, "drive.member.add", map[string]any{"spaceId": shared, "targetType": "account", "targetId": reader.id, "role": "viewer"})
	if contracts.Code(err) != "conflict" {
		t.Fatalf("duplicate membership must conflict: %v", err)
	}
	_, err = h.call(t, reader, "drive.member.add", map[string]any{"spaceId": shared, "targetType": "account", "targetId": outsider.id, "role": "admin"})
	if contracts.Code(err) != "not_found" {
		t.Fatalf("viewer must not grant membership: %v", err)
	}
	list := func(actor projectActor) []struct {
		ID   int64  `json:"id"`
		Role string `json:"role"`
	} {
		res := h.ok(t, actor, "drive.space.list", nil)
		return decodePayload[struct {
			Items []struct {
				ID   int64  `json:"id"`
				Role string `json:"role"`
			} `json:"items"`
		}](t, res.Data).Items
	}
	if items := list(reader); len(items) != 1 || items[0].ID != shared || items[0].Role != "viewer" {
		t.Fatalf("unexpected reader spaces: %+v", items)
	}
	if items := list(outsider); len(items) != 0 {
		t.Fatalf("outsider can see spaces: %+v", items)
	}
	if _, err := h.pool.Exec(context.Background(), `DELETE FROM drive_members WHERE tenant_id='tenant-a' AND space_id=$1 AND account_id=$2`, shared, reader.id); err != nil {
		t.Fatal(err)
	}
	if items := list(reader); len(items) != 0 {
		t.Fatalf("revoked reader can still see space: %+v", items)
	}
	_, err = h.callWith(t, owner, "drive.space.create", map[string]any{"name": "Agent"}, "agent-create", contracts.OriginAgent)
	if contracts.Code(err) != "forbidden" {
		t.Fatalf("automated creation must be rejected: %v", err)
	}
}

func TestDriveDepartmentAndAccountGrants(t *testing.T) {
	h := newDriveHarness(t)
	owner := h.account(t, "drive-department-owner", "drive:use", "drive:create")
	member := h.account(t, "drive-department-member", "drive:use")
	if _, err := h.pool.Exec(context.Background(), `UPDATE iam_accounts SET overrides='[{"permission":"drive:use","effect":"allow","scope":"ALL"}]'::jsonb WHERE tenant_id='tenant-a' AND id=$1`, owner.id); err != nil {
		t.Fatal(err)
	}
	space := rowID(t, h.ok(t, owner, "drive.space.create", map[string]any{"name": "Department"}))
	departmentGrant := rowID(t, h.ok(t, owner, "drive.member.add", map[string]any{"spaceId": space, "targetType": "department", "targetId": h.rootAccount.DepartmentID, "role": "viewer"}))
	accountGrant := rowID(t, h.ok(t, owner, "drive.member.add", map[string]any{"spaceId": space, "targetType": "account", "targetId": member.id, "role": "editor"}))
	role := func() string {
		t.Helper()
		items := decodePayload[struct {
			Items []struct {
				ID   int64  `json:"id"`
				Role string `json:"role"`
			} `json:"items"`
		}](t, h.ok(t, member, "drive.space.list", nil).Data).Items
		for _, item := range items {
			if item.ID == space {
				return item.Role
			}
		}
		return ""
	}
	if got := role(); got != "editor" {
		t.Fatalf("combined role = %q", got)
	}
	h.ok(t, member, "drive.folder.create", map[string]any{"spaceId": space, "name": "Editor allowed"})
	upload := rowID(t, h.ok(t, member, "drive.upload.start", map[string]any{"spaceId": space, "name": "pending.txt", "size": 4}))
	h.ok(t, owner, "drive.member.remove", map[string]any{"spaceId": space, "memberId": accountGrant})
	if got := role(); got != "viewer" {
		t.Fatalf("role after account grant removal = %q", got)
	}
	if _, err := h.call(t, member, "drive.folder.create", map[string]any{"spaceId": space, "name": "Viewer denied"}); contracts.Code(err) != "not_found" {
		t.Fatalf("viewer write accepted: %v", err)
	}
	digest := sha256.Sum256([]byte("data"))
	for _, test := range []struct {
		name  string
		input map[string]any
	}{
		{"drive.upload.status", map[string]any{"id": upload}},
		{"drive.upload.chunk", map[string]any{"spaceId": space, "uploadId": upload, "index": 0, "sha256": hex.EncodeToString(digest[:]), "content": base64.StdEncoding.EncodeToString([]byte("data"))}},
		{"drive.upload.complete", map[string]any{"spaceId": space, "uploadId": upload}},
	} {
		if _, err := h.call(t, member, test.name, test.input); contracts.Code(err) != "not_found" {
			t.Errorf("%s after revocation: %v", test.name, err)
		}
	}
	h.ok(t, owner, "drive.member.remove", map[string]any{"spaceId": space, "memberId": departmentGrant})
	if got := role(); got != "" {
		t.Fatalf("revoked department retains role %q", got)
	}
}

func TestDriveCrossTenantDirectIDs(t *testing.T) {
	h := newDriveHarness(t)
	owner := h.account(t, "drive-tenant-owner", "drive:use")
	space := rowID(t, h.ok(t, owner, "drive.space.personal.ensure", nil))
	folder := rowID(t, h.ok(t, owner, "drive.folder.create", map[string]any{"spaceId": space, "name": "Private"}))
	upload := rowID(t, h.ok(t, owner, "drive.upload.start", map[string]any{"spaceId": space, "name": "private", "size": 0}))
	file := rowID(t, h.ok(t, owner, "drive.upload.complete", map[string]any{"spaceId": space, "uploadId": upload}))
	account, err := identity.Bootstrap(context.Background(), h.pool, "tenant-b", "drive-other-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := h.iam.PrincipalForTask(context.Background(), "tenant-b", account.ID, account.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	other := projectActor{name: "other-tenant", id: account.ID, ctx: identity.WithPrincipal(context.Background(), principal)}
	for _, test := range []struct {
		name  string
		input map[string]any
	}{
		{"drive.space.usage", map[string]any{"spaceId": space}},
		{"drive.folder.path", map[string]any{"spaceId": space, "folderId": folder}},
		{"drive.folder.create", map[string]any{"spaceId": space, "name": "Denied"}},
		{"drive.upload.status", map[string]any{"id": upload}},
		{"drive.file.chunk.read", map[string]any{"spaceId": space, "fileId": file, "offset": 0}},
		{"drive.file.trash", map[string]any{"spaceId": space, "fileId": file}},
	} {
		if _, err := h.call(t, other, test.name, test.input); contracts.Code(err) != "not_found" {
			t.Errorf("%s cross-tenant result: %v", test.name, err)
		}
	}
}

func TestDriveLastEffectiveAdmin(t *testing.T) {
	h := newDriveHarness(t)
	owner := h.account(t, "drive-admin-owner", "drive:use", "drive:create")
	other := h.account(t, "drive-admin-other", "drive:use")
	if _, err := h.pool.Exec(context.Background(), `UPDATE iam_accounts SET overrides='[{"permission":"drive:use","effect":"allow","scope":"ALL"}]'::jsonb WHERE tenant_id='tenant-a' AND id IN ($1,$2)`, owner.id, other.id); err != nil {
		t.Fatal(err)
	}
	space := rowID(t, h.ok(t, owner, "drive.space.create", map[string]any{"name": "Admin handover"}))
	members := decodePayload[struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}](t,
		h.ok(t, owner, "drive.member.list", map[string]any{"spaceId": space}).Data)
	if len(members.Items) != 1 {
		t.Fatalf("expected creator grant: %+v", members.Items)
	}
	ownerGrant := members.Items[0].ID
	_, err := h.call(t, owner, "drive.member.remove", map[string]any{"spaceId": space, "memberId": ownerGrant})
	if contracts.Code(err) != "conflict" {
		t.Fatalf("last administrator removal must fail: %v", err)
	}
	otherGrant := rowID(t, h.ok(t, owner, "drive.member.add", map[string]any{"spaceId": space, "targetType": "account", "targetId": other.id, "role": "admin"}))
	h.ok(t, owner, "drive.member.remove", map[string]any{"spaceId": space, "memberId": ownerGrant})
	_, err = h.call(t, other, "drive.member.change", map[string]any{"spaceId": space, "memberId": otherGrant, "role": "viewer"})
	if contracts.Code(err) != "conflict" {
		t.Fatalf("last administrator demotion must fail: %v", err)
	}
}

func TestDriveUploadPolicyAndConcurrentQuota(t *testing.T) {
	h := newDriveHarness(t)
	owner := h.account(t, "drive-policy-owner", "drive:use")
	space := rowID(t, h.ok(t, owner, "drive.space.personal.ensure", nil))
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `INSERT INTO gov_scopes(tenant_id,scope_key,kind,entity_id) VALUES('tenant-a','workspace:0','workspace',0) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	set := func(key, value string) {
		t.Helper()
		if _, err := h.pool.Exec(ctx, `INSERT INTO gov_values(tenant_id,scope_key,config_key,value_json) VALUES('tenant-a','workspace:0',$1,$2::jsonb) ON CONFLICT(tenant_id,scope_key,config_key) DO UPDATE SET value_json=EXCLUDED.value_json`, key, value); err != nil {
			t.Fatal(err)
		}
	}
	set("drive.quotaBytes", "6")
	set("drive.blockedExtensions", `"exe"`)
	if _, err := h.call(t, owner, "drive.upload.start", map[string]any{"spaceId": space, "name": "blocked.EXE", "size": 1}); contracts.Code(err) != "invalid" {
		t.Fatalf("blocked extension accepted: %v", err)
	}
	results := make(chan error, 2)
	for _, name := range []string{"a.txt", "b.txt"} {
		go func(name string) {
			_, err := h.callWith(t, owner, "drive.upload.start", map[string]any{"spaceId": space, "name": name, "size": 4}, "quota-"+name, contracts.OriginHumanUI)
			results <- err
		}(name)
	}
	var succeededCount, conflictCount int
	for range 2 {
		err := <-results
		if err == nil {
			succeededCount++
		} else if contracts.Code(err) == "conflict" {
			conflictCount++
		} else {
			t.Fatalf("unexpected quota result: %v", err)
		}
	}
	if succeededCount != 1 || conflictCount != 1 {
		t.Fatalf("quota race: success=%d conflict=%d", succeededCount, conflictCount)
	}
	var upload int64
	if err := h.pool.QueryRow(ctx, `SELECT id FROM drive_uploads WHERE tenant_id='tenant-a' AND space_id=$1 AND state='open'`, space).Scan(&upload); err != nil {
		t.Fatal(err)
	}
	content := []byte("data")
	digest := sha256.Sum256(content)
	h.ok(t, owner, "drive.upload.chunk", map[string]any{"spaceId": space, "uploadId": upload, "index": 0, "sha256": hex.EncodeToString(digest[:]), "content": base64.StdEncoding.EncodeToString(content)})
	set("drive.blockedExtensions", `"txt"`)
	if _, err := h.call(t, owner, "drive.upload.complete", map[string]any{"spaceId": space, "uploadId": upload}); contracts.Code(err) != "invalid" {
		t.Fatalf("completion ignored changed extension policy: %v", err)
	}
	set("drive.blockedExtensions", `""`)
	set("drive.quotaBytes", "3")
	if _, err := h.call(t, owner, "drive.upload.complete", map[string]any{"spaceId": space, "uploadId": upload}); contracts.Code(err) != "conflict" {
		t.Fatalf("completion ignored reduced quota: %v", err)
	}
	set("drive.quotaBytes", "6")
	h.ok(t, owner, "drive.upload.complete", map[string]any{"spaceId": space, "uploadId": upload})
}

func TestDriveCleanupRequiresSystemAndPreservesCompletedFiles(t *testing.T) {
	h := newDriveHarness(t)
	owner := h.account(t, "drive-cleanup-owner", "drive:use", "drive:cleanup")
	space := rowID(t, h.ok(t, owner, "drive.space.personal.ensure", nil))
	upload := rowID(t, h.ok(t, owner, "drive.upload.start", map[string]any{"spaceId": space, "name": "empty", "size": 0}))
	file := rowID(t, h.ok(t, owner, "drive.upload.complete", map[string]any{"spaceId": space, "uploadId": upload}))
	_, err := h.callWith(t, owner, "drive.cleanup.upload", map[string]any{"spaceId": space, "uploadId": upload}, "user-cleanup", contracts.OriginEvent)
	if contracts.Code(err) != "forbidden" {
		t.Fatalf("ordinary account invoked system cleanup: %v", err)
	}
	principal, err := identity.NewPrincipal(&genidentity.AccountRecord{ID: 0, DepartmentID: 0, Status: "active"}, "tenant-a",
		[]*genidentity.PermissionGrant{{Permission: "drive:cleanup", Effect: "allow", Scope: "ALL"}})
	if err != nil {
		t.Fatal(err)
	}
	system := projectActor{name: "system", id: 0, ctx: identity.WithPrincipal(context.Background(), principal)}
	result, err := h.callWith(t, system, "drive.cleanup.upload", map[string]any{"spaceId": space, "uploadId": upload}, "system-upload", contracts.OriginEvent)
	succeeded(t, result, err)
	h.ok(t, owner, "drive.file.chunk.read", map[string]any{"spaceId": space, "fileId": file, "offset": 0})
	batch := rowID(t, h.ok(t, owner, "drive.file.trash", map[string]any{"spaceId": space, "fileId": file}))
	h.ok(t, owner, "drive.trash.purge.request", map[string]any{"spaceId": space, "batchId": batch})
	_, err = h.call(t, owner, "drive.trash.file.restore", map[string]any{"spaceId": space, "batchId": batch, "folderId": 0, "name": "empty"})
	if contracts.Code(err) != "not_found" {
		t.Fatalf("purging file was restorable: %v", err)
	}
	result, err = h.callWith(t, system, "drive.cleanup.batch", map[string]any{"spaceId": space, "batchId": batch}, "system-batch", contracts.OriginEvent)
	succeeded(t, result, err)
	var remaining int
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM drive_files WHERE tenant_id='tenant-a' AND id=$1`, file).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatal("purged file metadata remains")
	}
	folder := rowID(t, h.ok(t, owner, "drive.folder.create", map[string]any{"spaceId": space, "name": "Expired"}))
	expired := rowID(t, h.ok(t, owner, "drive.folder.trash", map[string]any{"spaceId": space, "folderId": folder}))
	var seconds int64
	if err = h.pool.QueryRow(context.Background(), `SELECT extract(epoch FROM expires_at-deleted_at)::bigint FROM drive_trash_batches WHERE tenant_id='tenant-a' AND id=$1`, expired).Scan(&seconds); err != nil {
		t.Fatal(err)
	}
	if seconds != 60*24*60*60 {
		t.Fatalf("unexpected retention: %d seconds", seconds)
	}
	if _, err = h.pool.Exec(context.Background(), `UPDATE drive_trash_batches SET expires_at=now()-interval '1 second' WHERE tenant_id='tenant-a' AND id=$1`, expired); err != nil {
		t.Fatal(err)
	}
	result, err = h.callWith(t, system, "drive.cleanup.claim", map[string]any{"spaceId": space, "batchId": expired}, "system-expired-claim", contracts.OriginEvent)
	succeeded(t, result, err)
	result, err = h.callWith(t, system, "drive.cleanup.batch", map[string]any{"spaceId": space, "batchId": expired}, "system-expired-batch", contracts.OriginEvent)
	succeeded(t, result, err)
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM drive_trash_batches WHERE tenant_id='tenant-a' AND id=$1`, expired).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatal("expired folder batch remains")
	}
}
