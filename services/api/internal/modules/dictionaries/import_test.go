package dictionaries

import (
	"strings"
	"testing"
)

func TestCSVFormulaProtectionRoundTripsLabels(t *testing.T) {
	for _, label := range []string{"=SUM(1,1)", "  =HYPERLINK(1)", "\n@command", "-1", "\tlabel", "'literal", "ordinary", "中文"} {
		encoded := formulaEscape(label)
		if formulaUnescape(encoded) != label {
			t.Fatal("label changed", label)
		}
		if strings.ContainsAny(strings.TrimLeft(label, " \t\r\n")[:1], "=+-@") && !strings.HasPrefix(encoded, "'") {
			t.Fatal("formula not escaped", label)
		}
	}
}
func TestImportRejectsDuplicateColumnsAndMalformedRows(t *testing.T) {
	for _, content := range []string{"code,code,labelEn\na,a,A\n", "code,labelEn\na,A,extra\n", "code,labelEn,sort\na,A,not-number\n"} {
		_, errors := parseImport("csv", content)
		if len(errors) == 0 {
			t.Fatal("invalid CSV accepted")
		}
	}
	_, errors := parseImport("json", `{"items":[]} {"items":[]}`)
	if len(errors) == 0 {
		t.Fatal("multiple JSON documents accepted")
	}
}
