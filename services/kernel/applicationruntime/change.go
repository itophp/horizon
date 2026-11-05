package applicationruntime

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// DefinitionChange addresses stable IDs, never array positions or object paths.
type DefinitionChange struct {
	Section   string `json:"section"`
	ID        string `json:"id"`
	Operation string `json:"operation"`
	Value     string `json:"value"`
}

func ApplyDefinitionChanges(source ApplicationV2, changes []DefinitionChange) (ApplicationV2, error) {
	if len(changes) == 0 || len(changes) > 100 {
		return source, fmt.Errorf("invalid")
	}
	out := Canonical(source)
	for _, change := range changes {
		if change.ID == "" || (change.Operation != "upsert" && change.Operation != "remove" && change.Operation != "order") {
			return source, fmt.Errorf("invalid")
		}
		var target any
		key := "ID"
		switch change.Section {
		case "pages":
			target = &out.Pages
		case "providers":
			target = &out.Providers
		case "navigation":
			target = &out.Navigation
			key = "PageID"
		case "messages":
			target = &out.Messages
			key = "Key"
		case "theme":
			target = &out.Theme
			key = "Name"
		default:
			return source, fmt.Errorf("field_not_allowed")
		}
		list := reflect.ValueOf(target).Elem()
		if change.Operation == "order" {
			var ids []string
			if json.Unmarshal([]byte(change.Value), &ids) != nil || len(ids) != list.Len() {
				return source, fmt.Errorf("invalid")
			}
			reordered := reflect.MakeSlice(list.Type(), 0, list.Len())
			seen := map[string]bool{}
			for _, id := range ids {
				if seen[id] {
					return source, fmt.Errorf("invalid")
				}
				seen[id] = true
				found := false
				for i := 0; i < list.Len(); i++ {
					if list.Index(i).FieldByName(key).String() == id {
						reordered = reflect.Append(reordered, list.Index(i))
						found = true
						break
					}
				}
				if !found {
					return source, fmt.Errorf("not_found")
				}
			}
			list.Set(reordered)
			continue
		}

		at := -1
		for i := 0; i < list.Len(); i++ {
			if list.Index(i).FieldByName(key).String() == change.ID {
				at = i
				break
			}
		}
		if change.Operation == "remove" {
			if at < 0 {
				return source, fmt.Errorf("not_found")
			}
			list.Set(reflect.AppendSlice(list.Slice(0, at), list.Slice(at+1, list.Len())))
			continue
		}
		item := reflect.New(list.Type().Elem())
		decoder := json.NewDecoder(strings.NewReader(change.Value))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(item.Interface()); err != nil {
			return source, fmt.Errorf("schema_invalid")
		}
		var trailing any
		if decoder.Decode(&trailing) != io.EOF {
			return source, fmt.Errorf("schema_invalid")
		}
		if item.Elem().FieldByName(key).String() != change.ID {
			return source, fmt.Errorf("invalid")
		}
		if at < 0 {
			list.Set(reflect.Append(list, item.Elem()))
		} else {
			list.Index(at).Set(item.Elem())
		}
	}
	raw, err := json.Marshal(out)
	if err != nil || len(raw) > 1048576 {
		return source, fmt.Errorf("invalid")
	}
	return out, nil
}
