package spec

import "baseline/services/api/kernel/capability/contracts"

// RuntimeDesigner can author proposals, but has no publication or lifecycle tools.
func RuntimeDesigner() Definition {
	return Definition{
		Name: "application-runtime-designer", Version: "1.0.0", ResultStep: "design", FreshContext: true,
		Input: contracts.Object(map[string]*contracts.Schema{"applicationId": contracts.Str(1, 100), "request": contracts.Str(1, 4000)}, "applicationId", "request"),
		Steps: []Step{{ID: "design", Agent: &Agent{Instruction: "Design a declarative application change for the supplied applicationId. Discover authorized contracts with definition.contracts: list names by kind, then inspect an exact name (capabilities use name@version). If applicationId is new, create a disabled application candidate with definition.create, using only registered contracts. Otherwise inspect its current summary, then inspect only the relevant page, form or provider; never request entire application snapshots. Preserve immutable identity and deployment ownership, then use stable-ID changes to propose the user's request. Never write scripts, SQL, code URLs, permissions or capability contracts. Use only existing authorized capability/component references. Reuse trusted deployed translation keys or declare new local labels as $app:key with matching en and zhCN message resources; never invent global translation keys. Do not publish or claim publication: the proposal requires human review. If unsupported or missing evidence, explain the limitation instead of inventing contracts. Return the actual candidate version and diagnostics from tools.", Tools: []string{"application.runtime.definition.contracts@1.0.0", "application.runtime.definition.create@1.0.0", "application.runtime.definition.list@1.0.0", "application.runtime.definition.inspect@1.0.0", "application.runtime.definition.propose@1.0.0", "application.runtime.definition.validate@1.0.0", "application.runtime.definition.diff@1.0.0"}, MaxIterations: 8}, Input: map[string]Mapping{"applicationId": {From: "input", Field: "applicationId"}, "request": {From: "input", Field: "request"}}}},
	}
}
