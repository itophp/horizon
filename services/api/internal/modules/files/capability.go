package files

import (
	"context"
	"fmt"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
)

// Event types published by the files module.
const (
	UploadedEvent = "file.uploaded"
	DeletedEvent  = "file.deleted"
)

const version = "1.0.0"

// Entries is the capability catalog contributed by the files module. The
// executors only delegate: authorization, validation and the transaction stay
// in the service methods, so the console and the agent share one path.
func (s *Service) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: s.listDefinition(), Handler: contracts.Handler{Execute: s.listFiles}},
		{Definition: s.driversDefinition(), Handler: contracts.Handler{Execute: s.readDrivers}},
		{Definition: s.uploadDefinition(), Handler: contracts.Handler{Execute: s.uploadFile}},
		{Definition: s.contentDefinition(), Handler: contracts.Handler{Execute: s.readContent}},
		{Definition: s.removeDefinition(), Handler: contracts.Handler{Execute: s.removeFile}},
	}
}

func fileIDSchema() *contracts.Schema {
	return contracts.Int(1, 1<<40)
}

func (s *Service) listDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "file.list.read", Version: version, Title: "List managed files",
		Description: "List the files the caller may see, newest first, with their storage link.",
		Domain:      "files", Entity: "file", Permission: "files:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:file.list", Tags: []string{"files", "read", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"files": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "name": {Type: "string"}, "mime": {Type: "string"},
				"size": {Type: "integer"}, "purpose": {Type: "string"}, "createdAt": {Type: "string"},
			}, "id", "name", "mime", "size", "purpose", "createdAt")},
		}),
	}
}

func (s *Service) driversDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "file.drivers.read", Version: version, Title: "List storage drivers",
		Description: "List the storage drivers compiled into the deployment and the settings keys each one reads.",
		Domain:      "files", Entity: "file", Permission: "files:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:file.drivers", Tags: []string{"files", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"drivers": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"code": {Type: "string"}, "writable": {Type: "boolean"},
				"configKeys": {Type: "array", Items: &contracts.Schema{Type: "string"}},
			}, "code", "writable", "configKeys")},
		}),
	}
}

func (s *Service) uploadDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "file.upload", Version: version, Title: "Upload a file",
		Description: "Store one file after the extension allowlist, size limit and content sniffing checks. Emits file.uploaded.",
		Domain:      "files", Entity: "file", Permission: "files:create",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{UploadedEvent}, Executor: "go:file.upload",
		Tags: []string{"files", "write", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"name":    contracts.Str(1, 200),
			"purpose": contracts.Enum("attachment", "logo"),
			// Base64 of the raw bytes: 29360128 characters decode to a 20 MiB file.
			"content": contracts.Str(1, 29360128),
		}, "name", "purpose", "content"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "name": {Type: "string"}, "mime": {Type: "string"},
			"size": {Type: "integer"}, "purpose": {Type: "string"}, "link": {Type: "string"},
		}, "id", "name", "mime", "size", "purpose"),
		// The bytes are never logged; only the descriptive fields.
		Loggable: []string{"name", "purpose"},
	}
}

func (s *Service) contentDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "file.content.read", Version: version, Title: "Read file content",
		Description: "Return the verified content of one file as base64 with its name and mime type.",
		Domain:      "files", Entity: "file", Permission: "files:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:file.content", Tags: []string{"files", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"id": fileIDSchema()}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"name": {Type: "string"}, "mime": {Type: "string"},
		}, "name", "mime"),
		Loggable: []string{"id"},
	}
}

func (s *Service) removeDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "file.delete", Version: version, Title: "Delete a file",
		Description: "Delete one file record and schedule its stored object for purge. High risk: a human must confirm before an agent runs it.",
		Domain:      "files", Entity: "file", Permission: "files:delete",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{DeletedEvent}, Executor: "go:file.delete",
		Tags:     []string{"files", "write"},
		Input:    contracts.Object(map[string]*contracts.Schema{"id": fileIDSchema()}, "id"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		Loggable: []string{"id"},
	}
}

func (s *Service) listFiles(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ListPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.List(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("files=%d", len(out))}, nil
}

func (s *Service) readDrivers(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DriversPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Drivers(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("drivers=%d", len(out))}, nil
}

func (s *Service) uploadFile(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := UploadPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Upload(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("file=%d", out.ID), Events: []contracts.Event{{
		Type: UploadedEvent, AggregateType: "file", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"fileId": out.ID, "purpose": out.Purpose, "size": out.Size, "mime": out.Mime},
	}}}, nil
}

func (s *Service) readContent(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ContentPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Content(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("file=%d", payload.ID)}, nil
}

func (s *Service) removeFile(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RemovePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.Remove(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    map[string]any{"id": payload.ID},
		Summary: fmt.Sprintf("file=%d", payload.ID),
		Events: []contracts.Event{{
			Type: DeletedEvent, AggregateType: "file", AggregateID: fmt.Sprintf("%d", payload.ID),
			Payload: map[string]any{"fileId": payload.ID},
		}},
	}, nil
}
