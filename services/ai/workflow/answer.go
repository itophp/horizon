// Package workflow holds deterministic Eino graphs. The first graph is grounded
// answering: retrieve, build a constrained prompt, then call the model. It is the
// building block future capability/workflow nodes will reuse.
package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// GroundingInstruction keeps answers inside the retrieved material.
const GroundingInstruction = "Answer only from the provided knowledge fragments. " +
	"If the fragments do not contain the answer, say so and name what is missing. " +
	"Cite the fragment title in parentheses. Answer in the user's language."

type questionKey struct{}

func withQuestion(ctx context.Context, question string) context.Context {
	return context.WithValue(ctx, questionKey{}, question)
}

func buildPrompt(ctx context.Context, docs []*schema.Document) ([]*schema.Message, error) {
	question, _ := ctx.Value(questionKey{}).(string)
	parts := make([]string, 0, len(docs))
	for _, doc := range docs {
		title, _ := doc.MetaData["title"].(string)
		parts = append(parts, fmt.Sprintf("[%s]\n%s", title, doc.Content))
	}
	material := strings.Join(parts, "\n\n")
	if material == "" {
		material = "(no matching knowledge)"
	}
	return []*schema.Message{
		schema.SystemMessage(GroundingInstruction),
		schema.UserMessage(fmt.Sprintf("知识库片段:\n%s\n\n用户问题: %s", material, question)),
	}, nil
}

// AnswerInput is the workflow input. A named struct (not a bare string) keeps
// the field mapping explicit: the retriever node reads Question from START.
type AnswerInput struct {
	Question string
}

// Answer runs the grounded answering workflow. The graph is built with Eino's
// Workflow composer: nodes are declared by dependency, and the framework wires
// the field mapping. Retrieval and the chat model are Eino components; only the
// prompt text is ours, because the grounding instruction is a product decision.
func Answer(ctx context.Context, cm model.BaseChatModel, r retriever.Retriever, question string) (string, error) {
	flow := compose.NewWorkflow[AnswerInput, *schema.Message]()
	flow.
		AddRetrieverNode("retrieve", r).
		AddInput(compose.START, compose.FromField("Question"))
	flow.
		AddLambdaNode("prompt", compose.InvokableLambda(buildPrompt)).
		AddInput("retrieve")
	flow.
		AddChatModelNode("answer", cm).
		AddInput("prompt")
	flow.End().AddInput("answer")

	runnable, err := flow.Compile(ctx)
	if err != nil {
		return "", err
	}
	message, err := runnable.Invoke(withQuestion(ctx, question), AnswerInput{Question: question})
	if err != nil {
		return "", err
	}
	if message == nil {
		return "", nil
	}
	return message.Content, nil
}
