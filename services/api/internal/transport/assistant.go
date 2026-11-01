package transport

import (
	genassistant "baseline/services/api/gen/assistant"
	assistantservice "baseline/services/api/internal/ai/assistant"
	"baseline/services/api/internal/ai/harness"
	"baseline/services/api/internal/ai/knowledge"
	"baseline/services/api/internal/governance"
	"context"
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
	return &genassistant.IAMProblem{Code: governance.Code(err)}
}

func (a *assistantAdapter) Agent(ctx context.Context, payload *genassistant.AgentPayload) (*genassistant.AssistantAgentReply, error) {
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
