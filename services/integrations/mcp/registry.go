package mcp

import (
	"context"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool inputs are operation-specific. Business values still flow through the
// existing generated service payloads and application validation in call.
type emptyInput struct{}
type articleCreateInput struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type articleSubmitInput struct {
	ID             int64  `json:"id"`
	Revision       int64  `json:"revision"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type workflowRunInput struct {
	Flow           string         `json:"flow"`
	Version        string         `json:"version"`
	Input          map[string]any `json:"input"`
	IdempotencyKey string         `json:"idempotencyKey"`
}
type workflowGetInput struct {
	RunID int64 `json:"runId"`
}
type workflowCancelInput struct {
	RunID          int64  `json:"runId"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type pageInput struct {
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}
type dictionaryInput struct {
	Code     string `json:"code"`
	Language string `json:"language"`
	Search   string `json:"search,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}
type notificationInput struct {
	ID int64 `json:"id"`
}
type readHandler func(context.Context, toolInput) (*sdk.CallToolResult, interface{}, error)

func registerReadTool(server *sdk.Server, name string, run readHandler) {
	tool := &sdk.Tool{Name: name, Description: descriptions[name], Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}}
	if name == "workflows_run" || name == "workflows_cancel" || name == "articles_create" || name == "articles_submit" {
		tool.Annotations.ReadOnlyHint = false
		destructive := true
		tool.Annotations.DestructiveHint = &destructive
	}
	switch name {
	case "articles_create":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in articleCreateInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{Title: in.Title, Body: in.Body, IdempotencyKey: in.IdempotencyKey})
		})
	case "articles_submit":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in articleSubmitInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{ID: in.ID, Revision: in.Revision, IdempotencyKey: in.IdempotencyKey})
		})
	case "articles_get":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in notificationInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{ID: in.ID})
		})

	case "workflows_run":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in workflowRunInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{Flow: in.Flow, Version: in.Version, Input: in.Input, IdempotencyKey: in.IdempotencyKey})
		})
	case "workflows_get":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in workflowGetInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{RunID: in.RunID})
		})
	case "workflows_cancel":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in workflowCancelInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{RunID: in.RunID, IdempotencyKey: in.IdempotencyKey})
		})
	case "context_get", "capabilities_list", "workflows_list", "articles_list":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, _ emptyInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{})
		})
	case "dictionary_options":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in dictionaryInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{Code: in.Code, Language: in.Language, Search: in.Search, Offset: in.Offset, Limit: in.Limit})
		})
	case "notifications_list":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in pageInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{Offset: in.Offset, Limit: in.Limit})
		})
	case "notifications_get":
		sdk.AddTool(server, tool, func(ctx context.Context, _ *sdk.CallToolRequest, in notificationInput) (*sdk.CallToolResult, interface{}, error) {
			return run(ctx, toolInput{ID: in.ID})
		})
	}
}
