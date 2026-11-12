package identity

import ar "horizon/foundation/kernel/applicationruntime"

// autoInput creates 1:1 field mappings for form commands where each form
// field maps directly to the capability input field with the same name.
func autoInput(fields ...string) []ar.Mapping {
	out := make([]ar.Mapping, len(fields))
	for i, f := range fields {
		if f == "id" {
			out[i] = ar.Mapping{Target: f, Value: ar.Path("record.id")}
		} else {
			out[i] = ar.Mapping{Target: f, Value: ar.Path("form." + f)}
		}
	}
	return out
}

// formFields creates FieldV2 definitions from simple name/type/component tuples.
func formFields(items ...[3]string) []ar.FieldV2 {
	out := make([]ar.FieldV2, len(items))
	for i, item := range items {
		out[i] = ar.FieldV2{
			Name: item[0], LabelKey: "admin:fields." + item[0],
			Type: item[1], Component: item[2], ComponentVersion: "2.0",
			Required: true,
		}
	}
	return out
}
