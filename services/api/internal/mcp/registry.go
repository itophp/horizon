package mcp

import (
	"context"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool inputs are operation-specific. Business values still flow through the
// existing generated service payloads and application validation in call.
type emptyInput struct{}
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
	switch name {
	case "context_get", "capabilities_list":
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
