package workbook

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"horizon/foundation/kernel/capability/contracts"
)

func TestParseProjectExamples(t *testing.T) {
	for _, example := range []struct {
		file, sheet, first string
	}{
		{"../../../apps/web/public/templates/projects-import.xlsx", "Projects", "000101"},
		{"testdata/projects-ai-import.xlsx", "项目周报", "WEEK-201"},
	} {
		t.Run(example.sheet, func(t *testing.T) {
			data, err := os.ReadFile(example.file)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := Parse(context.Background(), data, example.sheet)
			if err != nil || parsed.Sheet != example.sheet || len(parsed.Rows) != 4 {
				t.Fatalf("parse example: %+v, %v", parsed, err)
			}
			if !strings.Contains(parsed.Rows[1].Cells[0].Value, example.first) {
				t.Fatalf("unexpected first row: %+v", parsed.Rows[1])
			}
			if example.sheet == "Projects" {
				cells := parsed.Rows[1].Cells
				if len(cells) != 4 || cells[0].Value != "000101" || cells[3].Type != "date" || cells[3].Value != "2027-01-15" {
					t.Fatalf("code or date lost: %+v", cells)
				}
			}
		})
	}
}

func TestParseSelectedSheetAndCellTypes(t *testing.T) {
	book := workbookArchive(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="relationships"><sheets><sheet name="Other" r:id="a"/><sheet name="Selected" r:id="b"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="a" Type="test/worksheet" Target="worksheets/one.xml"/><Relationship Id="b" Type="test/worksheet" Target="/xl/worksheets/two.xml"/></Relationships>`,
		"xl/worksheets/one.xml":      `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>outside</t></is></c></row></sheetData></worksheet>`,
		"xl/worksheets/two.xml":      `<worksheet><sheetData><row r="3"><c r="A3" t="s"><v>0</v></c><c r="C3" t="inlineStr"><is><t>inline</t></is></c><c r="D3"><f>1+1</f><v>2</v></c></row></sheetData></worksheet>`,
		"xl/sharedStrings.xml":       `<sst><si><r><t>00</t></r><r><t>12</t></r></si></sst>`,
	})
	result, err := Parse(context.Background(), book, "Selected")
	if err != nil || len(result.Sheets) != 2 || len(result.Rows) != 1 {
		t.Fatalf("parse selected sheet: %+v, %v", result, err)
	}
	row := result.Rows[0]
	if row.Number != 3 || len(row.Cells) != 3 || row.Cells[0].Value != "0012" ||
		row.Cells[1].Address != "C3" || row.Cells[1].Value != "inline" ||
		row.Cells[2].Type != "formula" || row.Cells[2].Value != "" {
		t.Fatalf("unexpected cells: %+v", row)
	}
	if _, err := Parse(context.Background(), book, "Missing"); contracts.Code(err) != "excel_invalid" {
		t.Fatalf("unknown worksheet: %v", err)
	}
}

func TestParseRejectsCorruptAndOversizeWorkbooks(t *testing.T) {
	for _, test := range []struct {
		code string
		data []byte
	}{
		{"excel_invalid", []byte("invalid")},
		{"excel_limit", make([]byte, MaxFile+1)},
		{"excel_limit", workbookArchive(t, map[string]string{"xl/huge.xml": strings.Repeat("x", maxExpanded+1)})},
	} {
		if _, err := Parse(context.Background(), test.data, ""); contracts.Code(err) != test.code {
			t.Fatalf("expected %s, got %v", test.code, err)
		}
	}
}

func workbookArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var contents bytes.Buffer
	archive := zip.NewWriter(&contents)
	for name, content := range entries {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return contents.Bytes()
}
