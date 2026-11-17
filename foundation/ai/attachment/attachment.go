package attachment

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"

	"horizon/foundation/ai/workbook"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/files"
)

const (
	MaxBytes     = 2 << 20
	maxTextBytes = 12 << 10
	Invalid      = "ai_attachment_invalid"
)

type Upload struct {
	Name, Content string
}

type Image struct {
	MIME, Content string
}

func Read(ctx context.Context, upload Upload) (string, *Image, error) {
	if len(upload.Content) > base64.StdEncoding.EncodedLen(MaxBytes) || len([]rune(upload.Name)) > 255 {
		return "", nil, contracts.Problem(Invalid)
	}
	raw, err := base64.StdEncoding.DecodeString(upload.Content)
	if err != nil || len(raw) == 0 || len(raw) > MaxBytes {
		return "", nil, contracts.Problem(Invalid)
	}
	mime, err := files.ValidateAttachment(upload.Name, raw)
	if err != nil {
		return "", nil, contracts.Problem(Invalid)
	}
	var document string
	switch strings.ToLower(filepath.Ext(upload.Name)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return "", &Image{MIME: mime, Content: base64.StdEncoding.EncodeToString(raw)}, nil
	case ".docx":
		document, err = aiWordText(raw)
	case ".xlsx":
		document, err = aiWorkbookText(ctx, raw)
	case ".pdf":
		document, err = pdfText(ctx, raw)
	}
	if contracts.Code(err) == PDFUnavailable {
		return "", nil, err
	}
	if err != nil || strings.TrimSpace(document) == "" {
		return "", nil, contracts.Problem(Invalid)
	}
	return document, nil, nil
}

func aiWordText(raw []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", err
	}
	const xmlLimit = 4 << 20
	var document *zip.File
	for _, file := range archive.File {
		if file.Name != "word/document.xml" {
			continue
		}
		if document != nil || file.UncompressedSize64 > xmlLimit {
			return "", contracts.Problem(Invalid)
		}
		document = file
	}
	if document == nil {
		return "", contracts.Problem(Invalid)
	}
	file, err := document.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	reader := &io.LimitedReader{R: file, N: xmlLimit + 1}
	decoder := xml.NewDecoder(reader)
	root, err := decoder.Token()
	for err == nil {
		if _, ok := root.(xml.StartElement); ok {
			break
		}
		root, err = decoder.Token()
	}
	start, ok := root.(xml.StartElement)
	if err != nil || !ok || start.Name.Local != "document" ||
		(start.Name.Space != "http://schemas.openxmlformats.org/wordprocessingml/2006/main" &&
			start.Name.Space != "http://purl.oclc.org/ooxml/wordprocessingml/main") {
		return "", contracts.Problem(Invalid)
	}
	var out strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF && reader.N > 0 {
			return out.String(), nil
		}
		if err != nil {
			return "", contracts.Problem(Invalid)
		}
		switch element := token.(type) {
		case xml.StartElement:
			if element.Name.Space != start.Name.Space {
				continue
			}
			switch element.Name.Local {
			case "t":
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return "", err
				}
				out.WriteString(value)
			case "tab":
				out.WriteByte('\t')
			case "br", "cr":
				out.WriteByte('\n')
			}
		case xml.EndElement:
			if element.Name.Space == start.Name.Space && element.Name.Local == "p" && out.Len() > 0 {
				out.WriteByte('\n')
			}
		}
		if out.Len() > maxTextBytes {
			return "", contracts.Problem(Invalid)
		}
	}
}

func aiWorkbookText(ctx context.Context, raw []byte) (string, error) {
	sheet, err := workbook.Parse(ctx, raw, "")
	if err != nil {
		return "", err
	}
	if len(sheet.Sheets) > 8 {
		return "", contracts.Problem(Invalid)
	}
	var out strings.Builder
	for _, name := range sheet.Sheets {
		current := sheet
		if name != sheet.Sheet {
			current, err = workbook.Parse(ctx, raw, name)
			if err != nil {
				return "", err
			}
		}
		wroteHeading := false
		for _, row := range current.Rows {
			rowStart := out.Len()
			for _, cell := range row.Cells {
				if cell.Value == "" {
					continue
				}
				if !wroteHeading {
					out.WriteString(current.Sheet + "\n")
					wroteHeading = true
				}
				out.WriteString(cell.Address + ": " + cell.Value + "\t")
				if out.Len() >= maxTextBytes {
					return "", contracts.Problem(Invalid)
				}
			}
			if out.Len() > rowStart {
				out.WriteByte('\n')
			}
		}
	}
	return out.String(), nil
}
