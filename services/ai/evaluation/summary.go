// Package evaluation turns the run trace into the numbers an operator needs:
// completion rate, failure rate, latency and token spend.
package evaluation

import "baseline/services/api/internal/repository/dbgen"

// Summary is the tenant level agent quality snapshot.
type Summary struct {
	Total             int64
	Completed         int64
	Failed            int64
	PromptTokens      int64
	CompletionTokens  int64
	AverageDurationMs int64
}

// Rate is the completed share of all runs, 0 when nothing ran yet.
func (s Summary) Rate() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Completed) / float64(s.Total)
}

// FromRow projects the database aggregate.
func FromRow(row dbgen.AIEvaluationRow) Summary {
	return Summary{
		Total: row.Total, Completed: row.Completed, Failed: row.Failed,
		PromptTokens: row.PromptTokens, CompletionTokens: row.CompletionTokens,
		AverageDurationMs: row.AverageDurationMs,
	}
}
