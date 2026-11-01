package orchestration

import (
	"baseline/services/api/internal/ai/orchestration/spec"
	js "github.com/google/jsonschema-go/jsonschema"
)

// Runtime aliases preserve the API while pure specs stay independent of
// application services and generated SQL/Goa code, so generation can run first.
type Definition = spec.Definition
type Step = spec.Step
type Mapping = spec.Mapping
type Poll = spec.Poll
type Agent = spec.Agent
type Compensation = spec.Compensation

const (
	IdentifierPattern  = spec.IdentifierPattern
	MaxWorkflowSteps   = spec.MaxWorkflowSteps
	MaxReadAttempts    = spec.MaxReadAttempts
	MaxPollSteps       = spec.MaxPollSteps
	MaxAgentIterations = spec.MaxAgentIterations
	MaxVersionLength   = spec.MaxVersionLength
	MaxDefinitionBytes = spec.MaxDefinitionBytes
)

func Builtins() []Definition                          { return spec.Builtins() }
func LifecycleExample() Definition                    { return spec.LifecycleExample() }
func SpecificationSchema() (*js.Schema, error)        { return spec.SpecificationSchema() }
func DecodeDefinition(raw []byte) (Definition, error) { return spec.DecodeDefinition(raw) }
