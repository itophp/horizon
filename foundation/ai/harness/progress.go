package harness

import (
	"context"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"io"
)

// Progress contains public stages, never hidden reasoning or raw tool data.
type Progress struct {
	Kind, Delta, Capability, Status, ErrorCode string
	Run                                        *Run
}
type progressKey struct{}
type progressHandler func(Progress)

func emitProgress(ctx context.Context, event Progress) {
	if handler, ok := ctx.Value(progressKey{}).(progressHandler); ok {
		handler(event)
	}
}

// RunStream uses exactly the same authorized agent path as Run.
func (s *Service) RunStream(ctx context.Context, in RunInput, emit func(Progress)) (*Run, error) {
	return s.Run(context.WithValue(ctx, progressKey{}, progressHandler(emit)), in)
}
func publicMessage(ctx context.Context, output *adk.MessageVariant) (*schema.Message, error) {
	if !output.IsStreaming {
		return output.GetMessage()
	}
	defer output.MessageStream.Close()
	var chunks []*schema.Message
	for {
		chunk, err := output.MessageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		if chunk.Content != "" {
			emitProgress(ctx, Progress{Kind: "answer_delta", Delta: chunk.Content})
		}
	}
	return schema.ConcatMessages(chunks)
}
