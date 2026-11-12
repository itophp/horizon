package identity

import "testing"

func TestLowerCamelMatchesGoaJSONNames(t *testing.T) {
	for input, want := range map[string]string{
		"ID":             "id",
		"OrganizationID": "organizationId",
		"DepartmentID":   "departmentId",
		"RoleIds":        "roleIds",
		"IsAdmin":        "isAdmin",
		"FullName":       "fullName",
		"CreatedAt":      "createdAt",
		"LastLogin":      "lastLogin",
		"MemberCount":    "memberCount",
		"Permission":     "permission",
	} {
		if got := lowerCamel(input); got != want {
			t.Fatalf("%s -> %s, want %s", input, got, want)
		}
	}
}

func TestCamelizeKeepsNestedRecordsReadable(t *testing.T) {
	projected := camelize(map[string]any{
		"ID":             int64(1),
		"OrganizationID": int64(2),
		"Memberships":    []any{map[string]any{"DepartmentID": int64(3), "RoleIds": []any{int64(4)}}},
	})
	if projected.(map[string]any)["id"] != int64(1) {
		t.Fatal("top level id missing")
	}
	membership := projected.(map[string]any)["memberships"].([]any)[0].(map[string]any)
	if membership["departmentId"] != int64(3) {
		t.Fatal("nested department id missing")
	}
}
