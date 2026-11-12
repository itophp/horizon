package executionplans

import "horizon/foundation/kernel/capability/contracts"

func planSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"planId": {Type: "string"}, "capabilityId": {Type: "string"}, "capabilityVersion": {Type: "string"}, "version": {Type: "integer"}, "mode": {Type: "string", Enum: []string{"direct", "workflow", "graph"}}, "implementationRef": {Type: "string"}, "status": {Type: "string", Enum: []string{"draft", "validated", "published", "archived"}}, "workflowHash": {Type: "string"}, "graphHash": {Type: "string"},
	}, "planId", "capabilityId", "capabilityVersion", "version", "mode", "implementationRef", "status")
}
func bindingSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{"bindingId": {Type: "string"}, "scopeType": {Type: "string", Enum: []string{"platform", "tenant"}}, "scopeId": {Type: "string"}, "revision": {Type: "integer"}, "plan": planSchema()}, "bindingId", "scopeType", "revision", "plan")
}
func genericObject() *contracts.Schema {
	return &contracts.Schema{Type: "object", AdditionalProperties: true}
}
func array(s *contracts.Schema) *contracts.Schema { return &contracts.Schema{Type: "array", Items: s} }
func outputSchema(action string) *contracts.Schema {
	text := func() *contracts.Schema { return &contracts.Schema{Type: "string"} }
	switch action {
	case "list":
		return contracts.Object(map[string]*contracts.Schema{"plans": array(contracts.Object(map[string]*contracts.Schema{"plan": planSchema(), "name": text(), "scopeType": {Type: "string", Enum: []string{"platform", "tenant"}}, "createdBy": {Type: "integer"}, "createdAt": text(), "publishedBy": {Type: "integer"}, "publishedAt": text()}, "plan", "name", "scopeType", "createdBy", "createdAt", "publishedBy"))}, "plans")
	case "binding":
		return contracts.Object(map[string]*contracts.Schema{"binding": bindingSchema()}, "binding")
	case "history":
		return contracts.Object(map[string]*contracts.Schema{"changes": array(contracts.Object(map[string]*contracts.Schema{"ID": {Type: "integer"}, "PlanID": text(), "PlanVersion": {Type: "integer"}, "Revision": {Type: "integer"}, "Action": text(), "Reason": text(), "AccountID": {Type: "integer"}, "CreatedAt": text()}, "ID", "PlanID", "PlanVersion", "Revision", "Action", "Reason", "AccountID", "CreatedAt"))}, "changes")
	case "records":
		return contracts.Object(map[string]*contracts.Schema{"records": array(contracts.Object(map[string]*contracts.Schema{"invocationId": text(), "binding": bindingSchema(), "runId": {Type: "integer"}, "runStatus": text(), "status": text(), "traceId": text(), "causationId": text(), "startedAt": text(), "finishedAt": text(), "errorCode": text()}, "invocationId", "binding", "runStatus", "status", "traceId", "causationId", "startedAt", "errorCode"))}, "records")
	case "references":
		return contracts.Object(map[string]*contracts.Schema{"references": array(contracts.Object(map[string]*contracts.Schema{"ref": text(), "name": text(), "version": text(), "mode": {Type: "string", Enum: []string{"workflow", "graph"}}, "hash": text()}, "ref", "name", "version", "mode", "hash"))}, "references")
	case "inspect":
		return contracts.Object(map[string]*contracts.Schema{"inputSchema": genericObject(), "outputSchema": genericObject(), "diagram": genericObject(), "referenceHash": text(), "budget": genericObject()}, "inputSchema", "referenceHash")
	case "preflight":
		return contracts.Object(map[string]*contracts.Schema{"valid": {Type: "boolean"}, "effectsExecuted": {Type: "boolean"}}, "valid", "effectsExecuted")
	case "trace":
		return contracts.Object(map[string]*contracts.Schema{"events": array(contracts.Object(map[string]*contracts.Schema{"sequence": {Type: "integer"}, "stepName": text(), "eventType": {Type: "string", Enum: []string{"STEP_STARTED", "STEP_FINISHED"}}, "occurredAt": text()}, "sequence", "stepName", "eventType", "occurredAt"))}, "events")
	default:
		return contracts.Object(map[string]*contracts.Schema{"planId": text(), "planVersion": {Type: "integer"}}, "planId", "planVersion")
	}
}
