package applicationruntime

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// RebaseDefinition merges immutable deployment baselines with tenant changes.
// Stable declaration IDs are used for arrays; primitive arrays are indivisible.
func RebaseDefinition(base, current, next ApplicationV2) (ApplicationV2, []string, error) {
	object := func(app ApplicationV2) (map[string]any, error) {
		raw, err := json.Marshal(app)
		if err != nil {
			return nil, err
		}
		out := map[string]any{}
		err = json.Unmarshal(raw, &out)
		return out, err
	}
	b, err := object(base)
	if err != nil {
		return next, nil, err
	}
	c, err := object(current)
	if err != nil {
		return next, nil, err
	}
	n, err := object(next)
	if err != nil {
		return next, nil, err
	}
	conflicts := []string{}
	var merge func(any, any, any, string) any
	merge = func(old, custom, newValue any, path string) any {
		if reflect.DeepEqual(custom, old) {
			return newValue
		}
		if reflect.DeepEqual(newValue, old) || reflect.DeepEqual(custom, newValue) {
			return custom
		}
		om, ook := old.(map[string]any)
		cm, cok := custom.(map[string]any)
		nm, nok := newValue.(map[string]any)
		if ook && cok && nok {
			keys := map[string]bool{}
			for k := range om {
				keys[k] = true
			}
			for k := range cm {
				keys[k] = true
			}
			for k := range nm {
				keys[k] = true
			}
			out := map[string]any{}
			names := []string{}
			for key := range keys {
				names = append(names, key)
			}
			sort.Strings(names)
			for _, key := range names {
				value := merge(om[key], cm[key], nm[key], path+"."+key)
				if value != nil {
					out[key] = value
				}
			}
			return out
		}
		oa, ook := old.([]any)
		ca, cok := custom.([]any)
		na, nok := newValue.([]any)
		if ook && cok && nok {
			key := ""
			for _, candidate := range []string{"id", "name", "pageId", "target", "key"} {
				valid := true
				for _, list := range [][]any{oa, ca, na} {
					seen := map[string]bool{}
					for _, item := range list {
						row, ok := item.(map[string]any)
						if !ok {
							valid = false
							break
						}
						id, ok := row[candidate].(string)
						if !ok || id == "" || seen[id] {
							valid = false
							break
						}
						seen[id] = true
					}
				}
				if valid {
					key = candidate
					break
				}
			}
			if key != "" {
				index := func(list []any) (map[string]any, []string) {
					rows := map[string]any{}
					order := []string{}
					for _, item := range list {
						row := item.(map[string]any)
						id := row[key].(string)
						rows[id] = item
						order = append(order, id)
					}
					return rows, order
				}
				oi, oo := index(oa)
				ci, co := index(ca)
				ni, no := index(na)
				common := map[string]bool{}
				for _, id := range oo {
					if ci[id] != nil && ni[id] != nil {
						common[id] = true
					}
				}
				relative := func(order []string) []string {
					out := []string{}
					for _, id := range order {
						if common[id] {
							out = append(out, id)
						}
					}
					return out
				}
				oldOrder, customOrder, newOrder := relative(oo), relative(co), relative(no)
				order := append([]string{}, no...)
				if !reflect.DeepEqual(customOrder, oldOrder) {
					if !reflect.DeepEqual(newOrder, oldOrder) && !reflect.DeepEqual(customOrder, newOrder) {
						conflicts = append(conflicts, path+".order")
					} else {
						order = append([]string{}, co...)
					}
				}
				present := map[string]bool{}
				for _, id := range order {
					present[id] = true
				}
				for _, list := range [][]string{co, no} {
					for _, id := range list {
						if !present[id] {
							order = append(order, id)
							present[id] = true
						}
					}
				}
				out := []any{}
				for _, id := range order {
					value := merge(oi[id], ci[id], ni[id], path+"["+id+"]")
					if value != nil {
						out = append(out, value)
					}
				}
				return out
			}
		}
		conflicts = append(conflicts, path)
		return custom
	}
	for _, field := range []string{"labelKey", "permission", "route", "dependencies", "protocol", "messages", "theme"} {
		n[field] = merge(b[field], c[field], n[field], field)
	}
	n["id"] = current.ID
	n["source"] = current.Source
	n["metadataVersion"] = ""
	n["definitionDigest"] = ""
	n["definitionRevision"] = float64(0)
	n["previewVersion"] = ""
	raw, err := json.Marshal(n)
	if err != nil {
		return next, nil, err
	}
	var result ApplicationV2
	if err = json.Unmarshal(raw, &result); err != nil {
		return next, nil, err
	}
	sort.Strings(conflicts)
	if len(raw) > 1048576 {
		return next, nil, fmt.Errorf("invalid")
	}
	return result, conflicts, nil
}
