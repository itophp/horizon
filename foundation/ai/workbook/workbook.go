// Package workbook extracts a bounded, data-only view of an XLSX worksheet.
package workbook

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"horizon/foundation/kernel/capability/contracts"
)

const (
	ParseCapability = "document.excel.parse"
	MaxFile         = 2 << 20
	MaxRows         = 1000
	MaxCell         = 4096
	maxColumns      = 64
	maxExpanded     = 16 << 20
	maxText         = 512 << 10
)

type Cell struct {
	Address string `json:"address"`
	Value   string `json:"value"`
	Type    string `json:"type"`
}

type Row struct {
	Number int    `json:"number"`
	Cells  []Cell `json:"cells"`
}

type Result struct {
	Sheets []string `json:"sheets"`
	Sheet  string   `json:"sheet"`
	Rows   []Row    `json:"rows"`
}

type workbookXML struct {
	XMLName    xml.Name `xml:"workbook"`
	Properties struct {
		Date1904 bool `xml:"date1904,attr"`
	} `xml:"workbookPr"`
	Sheets []struct {
		Name string `xml:"name,attr"`
		ID   string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}

type richText struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func (r richText) value() string {
	var out strings.Builder
	out.WriteString(r.Text)
	for _, run := range r.Runs {
		out.WriteString(run.Text)
	}
	return out.String()
}

type sheetCell struct {
	Address string    `xml:"r,attr"`
	Type    string    `xml:"t,attr"`
	Style   int       `xml:"s,attr"`
	Value   string    `xml:"v"`
	Inline  richText  `xml:"is"`
	Formula *struct{} `xml:"f"`
}

func Parse(ctx context.Context, data []byte, sheet string) (Result, error) {
	if len(data) > MaxFile {
		return Result{}, contracts.Problem("excel_limit")
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Result{}, contracts.Problem("excel_invalid")
	}
	if len(archive.File) > 256 {
		return Result{}, contracts.Problem("excel_limit")
	}
	files := make(map[string]*zip.File, len(archive.File))
	var expanded uint64
	for _, file := range archive.File {
		if file.UncompressedSize64 > maxExpanded-expanded {
			return Result{}, contracts.Problem("excel_limit")
		}
		expanded += file.UncompressedSize64
		if files[file.Name] != nil {
			return Result{}, contracts.Problem("excel_invalid")
		}
		files[file.Name] = file
	}

	var book workbookXML
	var relations struct {
		Links []struct {
			ID     string `xml:"Id,attr"`
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if readXML(files["xl/workbook.xml"], &book) != nil ||
		readXML(files["xl/_rels/workbook.xml.rels"], &relations) != nil {
		return Result{}, contracts.Problem("excel_invalid")
	}
	targets := map[string]string{}
	for _, link := range relations.Links {
		if !strings.HasSuffix(link.Type, "/worksheet") || link.Mode == "External" {
			continue
		}
		target := path.Clean(path.Join("xl", link.Target))
		if strings.HasPrefix(link.Target, "/") {
			target = path.Clean(strings.TrimPrefix(link.Target, "/"))
		}
		if strings.HasPrefix(target, "xl/worksheets/") && files[target] != nil {
			targets[link.ID] = target
		}
	}
	result := Result{Rows: []Row{}}
	var selected string
	for _, candidate := range book.Sheets {
		target := targets[candidate.ID]
		if target == "" || candidate.Name == "" || utf8.RuneCountInString(candidate.Name) > 31 {
			continue
		}
		result.Sheets = append(result.Sheets, candidate.Name)
		if selected == "" && (sheet == "" || sheet == candidate.Name) {
			result.Sheet, selected = candidate.Name, target
		}
	}
	if selected == "" {
		return Result{}, contracts.Problem("excel_invalid")
	}

	var shared struct {
		Items []richText `xml:"si"`
	}
	if file := files["xl/sharedStrings.xml"]; file != nil && readXML(file, &shared) != nil {
		return Result{}, contracts.Problem("excel_invalid")
	}
	dates, err := dateStyles(files["xl/styles.xml"])
	if err != nil {
		return Result{}, contracts.Problem("excel_invalid")
	}
	reader, err := files[selected].Open()
	if err != nil {
		return Result{}, contracts.Problem("excel_invalid")
	}
	defer reader.Close()
	decoder := xml.NewDecoder(io.LimitReader(reader, maxExpanded+1))
	textBytes := 0
	lastRow := 0
	for {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return Result{}, contracts.Problem("excel_invalid")
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}
		var raw struct {
			Number int         `xml:"r,attr"`
			Cells  []sheetCell `xml:"c"`
		}
		if decoder.DecodeElement(&raw, &start) != nil {
			return Result{}, contracts.Problem("excel_invalid")
		}
		if raw.Number > MaxRows || len(raw.Cells) > maxColumns {
			return Result{}, contracts.Problem("excel_limit")
		}
		if raw.Number <= lastRow {
			return Result{}, contracts.Problem("excel_invalid")
		}
		lastRow = raw.Number
		row := Row{Number: raw.Number, Cells: []Cell{}}
		seen := map[string]bool{}
		for _, item := range raw.Cells {
			column, number := coordinate(item.Address)
			if column > maxColumns {
				return Result{}, contracts.Problem("excel_limit")
			}
			if column == 0 || number != raw.Number || seen[item.Address] {
				return Result{}, contracts.Problem("excel_invalid")
			}
			seen[item.Address] = true
			cell, err := extractCell(item, shared.Items, dates[item.Style], book.Properties.Date1904)
			if err != nil {
				return Result{}, err
			}
			textBytes += len(cell.Value)
			if len(cell.Value) > MaxCell || textBytes > maxText {
				return Result{}, contracts.Problem("excel_limit")
			}
			if cell.Value != "" || cell.Type == "formula" || cell.Type == "error" {
				row.Cells = append(row.Cells, cell)
			}
		}
		if len(row.Cells) > 0 {
			result.Rows = append(result.Rows, row)
		}
	}
}

func readXML(file *zip.File, target any) error {
	if file == nil {
		return io.ErrUnexpectedEOF
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	return xml.NewDecoder(io.LimitReader(reader, maxExpanded+1)).Decode(target)
}

var addressPattern = regexp.MustCompile(`^([A-Z]{1,3})([1-9][0-9]{0,6})$`)

func coordinate(address string) (column, row int) {
	parts := addressPattern.FindStringSubmatch(address)
	if parts == nil {
		return 0, 0
	}
	for _, letter := range parts[1] {
		column = column*26 + int(letter-'A'+1)
	}
	row, _ = strconv.Atoi(parts[2])
	return
}

func extractCell(raw sheetCell, shared []richText, date, date1904 bool) (Cell, error) {
	cell := Cell{Address: raw.Address, Value: raw.Value, Type: "text"}
	switch {
	case raw.Formula != nil:
		cell.Type, cell.Value = "formula", ""
	case raw.Type == "e":
		cell.Type, cell.Value = "error", ""
	case raw.Type == "inlineStr":
		cell.Value = raw.Inline.value()
	case raw.Type == "s":
		index, err := strconv.Atoi(raw.Value)
		if err != nil || index < 0 || index >= len(shared) {
			return Cell{}, contracts.Problem("excel_invalid")
		}
		cell.Value = shared[index].value()
	case raw.Type == "b":
		cell.Type = "boolean"
		cell.Value = strconv.FormatBool(raw.Value == "1")
	case raw.Type == "d":
		cell.Type = "date"
		if len(cell.Value) > 10 {
			cell.Value = cell.Value[:10]
		}
	case raw.Value != "" && (raw.Type == "" || raw.Type == "n"):
		cell.Type = "number"
		if date {
			serial, err := strconv.ParseFloat(raw.Value, 64)
			if err != nil || !(serial >= 0 && serial < 2958466) || !date1904 && int(serial) == 60 {
				cell.Type, cell.Value = "error", ""
				break
			}
			base := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
			if date1904 {
				base = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
			} else if serial >= 60 {
				serial--
			}
			cell.Type, cell.Value = "date", base.AddDate(0, 0, int(serial)).Format(time.DateOnly)
		}
	}
	return cell, nil
}

var formatLiterals = regexp.MustCompile(`"[^"]*"|\\.|\[[^\]]*\]`)

func dateStyles(file *zip.File) (map[int]bool, error) {
	result := map[int]bool{}
	if file == nil {
		return result, nil
	}
	var styles struct {
		Formats []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Cells []struct {
			Format int `xml:"numFmtId,attr"`
		} `xml:"cellXfs>xf"`
	}
	if err := readXML(file, &styles); err != nil {
		return nil, err
	}
	custom := map[int]bool{}
	for _, format := range styles.Formats {
		code := strings.ToLower(formatLiterals.ReplaceAllString(format.Code, ""))
		custom[format.ID] = strings.ContainsAny(code, "yd")
	}
	for index, cell := range styles.Cells {
		id := cell.Format
		result[index] = custom[id] || id >= 14 && id <= 17 || id == 22 ||
			id >= 27 && id <= 31 || id >= 34 && id <= 36 || id >= 50 && id <= 58
	}
	return result, nil
}
