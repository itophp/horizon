package transport

import (
	"context"
	"encoding/json"
	assistantservice "horizon/foundation/ai/assistant"
	"horizon/foundation/ai/harness"
	"horizon/foundation/ai/knowledge"
	genassistant "horizon/foundation/gen/assistant"
)

// assistantAdapter exposes the in-page chat gateway, the capability driven agent
// and knowledge management through one generated service interface.
type assistantAdapter struct {
	*assistantservice.Service
	agent     *harness.Service
	documents *knowledge.Service
}

func newAssistant(service *assistantservice.Service, agent *harness.Service, documents *knowledge.Service) *assistantAdapter {
	return &assistantAdapter{Service: service, agent: agent, documents: documents}
}

func problem(err error) error {
	if err == nil {
		return nil
	}
	return &genassistant.IAMProblem{Code: aiErrorCode(err)}
}

func (a *assistantAdapter) Agent(ctx context.Context, payload *genassistant.AgentPayload) (*genassistant.AssistantAgentReply, error) {
	if payload.SessionID != nil || payload.RunID != nil || payload.Tools != nil {
		value := func(p *string) string {
			if p != nil {
				return *p
			}
			return ""
		}
		in := harness.BrowserTurn{Question: payload.Question, SessionID: value(payload.SessionID), Token: value(payload.ResumeToken), CallID: value(payload.ToolCallID), Result: value(payload.ToolResult)}
		if payload.Page != nil {
			in.Page.ApplicationID = valueString(payload.Page.ApplicationID)
			in.Page.PageID = valueString(payload.Page.PageID)
			in.Page.MetadataVersion = valueString(payload.Page.MetadataVersion)
			in.Page.Route = valueString(payload.Page.Route)
			in.Page.Entity = valueString(payload.Page.Entity)
			if payload.Page.EntityID != nil {
				in.Page.EntityID = *payload.Page.EntityID
			}
			in.Page.Selection = payload.Page.Selection
		}
		if payload.RunID != nil {
			in.RunID = *payload.RunID
		}
		for _, t := range payload.Tools {
			if t == nil {
				return nil, &genassistant.IAMProblem{Code: "invalid"}
			}
			parameters, err := json.Marshal(t.Parameters)
			if err != nil {
				return nil, &genassistant.IAMProblem{Code: "invalid"}
			}
			in.Tools = append(in.Tools, harness.BrowserTool{Name: t.Name, Description: t.Description, Parameters: parameters})
		}
		reply, err := a.agent.AskBrowser(ctx, in)
		if err != nil {
			return nil, problem(err)
		}
		out := &genassistant.AssistantAgentReply{Content: reply.Content, Tools: reply.Tools, Model: reply.Model, Mode: reply.Mode, RunID: &reply.RunID}
		if reply.Call != nil {
			out.ResumeToken = &reply.Token
			out.PageCall = &genassistant.AssistantToolCall{ID: reply.Call.ID, Name: reply.Call.Name, Arguments: reply.Call.Arguments}
		}
		return out, nil
	}
	reply, err := a.agent.Ask(ctx, payload.Question)
	if err != nil {
		return nil, problem(err)
	}
	return &genassistant.AssistantAgentReply{Content: reply.Content, Tools: reply.Tools, Model: reply.Model, Mode: reply.Mode}, nil
}

func (a *assistantAdapter) Workflow(ctx context.Context, payload *genassistant.WorkflowPayload) (*genassistant.AssistantAgentReply, error) {
	reply, err := a.agent.Answer(ctx, payload.Question)
	if err != nil {
		return nil, problem(err)
	}
	return &genassistant.AssistantAgentReply{Content: reply.Content, Tools: reply.Tools, Model: reply.Model, Mode: reply.Mode}, nil
}

func (a *assistantAdapter) Knowledge(ctx context.Context, _ *genassistant.KnowledgePayload) ([]*genassistant.KnowledgeDocument, error) {
	rows, err := a.documents.Documents(ctx)
	if err != nil {
		return nil, problem(err)
	}
	out := make([]*genassistant.KnowledgeDocument, 0, len(rows))
	for _, row := range rows {
		out = append(out, &genassistant.KnowledgeDocument{
			ID: row.ID, Title: row.Title, Source: row.Source, Chunks: row.Chunks, CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func (a *assistantAdapter) Ingest(ctx context.Context, payload *genassistant.IngestPayload) (*genassistant.KnowledgeIngested, error) {
	id, err := a.documents.Ingest(ctx, payload.Title, payload.Content)
	if err != nil {
		return nil, problem(err)
	}
	return &genassistant.KnowledgeIngested{ID: id}, nil
}
