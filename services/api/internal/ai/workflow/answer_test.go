package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type stubRetriever struct{ query string }

func (s *stubRetriever) Retrieve(_ context.Context, query string, _ ...retriever.Option) ([]*schema.Document, error) {
	s.query = query
	return []*schema.Document{
		{
			ID:      "7",
			Content: "Staff must use the corporate VPN.",
			MetaData: map[string]any{
				"title": "Remote access policy",
			},
		},
	}, nil
}

type stubModel struct{ seen []*schema.Message }

func (m *stubModel) Generate(_ context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.seen = in
	return schema.AssistantMessage("grounded answer", nil), nil
}

func (m *stubModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream is not used here")
}

// The graph is wired by dependency, so this also proves the field mapping from
// the workflow input into the retriever node.
func TestAnswerRunsTheGroundedWorkflow(t *testing.T) {
	docs := &stubRetriever{}
	llm := &stubModel{}

	answer, err := Answer(context.Background(), llm, docs, "how do I connect?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if answer != "grounded answer" {
		t.Fatalf("answer = %q", answer)
	}
	if docs.query != "how do I connect?" {
		t.Fatalf("retriever received %q", docs.query)
	}
	if len(llm.seen) != 2 {
		t.Fatalf("model saw %d messages", len(llm.seen))
	}
	if llm.seen[0].Role != schema.System {
		t.Fatalf("first message role = %v", llm.seen[0].Role)
	}
	prompt := llm.seen[1].Content
	for _, want := range []string{"Remote access policy", "corporate VPN", "how do I connect?"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt is missing %q: %s", want, prompt)
		}
	}
}
