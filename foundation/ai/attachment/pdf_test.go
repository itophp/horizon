package attachment

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestReadPDFText(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext is not installed")
	}
	const text = "Revenue in July was 420."
	content := "BT /F1 12 Tf 30 100 Td (" + text + ") Tj ET\n"
	objects := []string{
		"<</Type /Catalog /Pages 2 0 R>>",
		"<</Type /Pages /Kids [3 0 R] /Count 1>>",
		"<</Type /Page /Parent 2 0 R /MediaBox [0 0 240 160] /Contents 4 0 R /Resources <</Font <</F1 5 0 R>>>>>>",
		fmt.Sprintf("<</Length %d>>\nstream\n%sendstream", len(content), content),
		"<</Type /Font /Subtype /Type1 /BaseFont /Helvetica>>",
	}
	var pdf strings.Builder
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<</Size %d /Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	document, image, err := Read(t.Context(), Upload{
		Name: "revenue.pdf", Content: base64.StdEncoding.EncodeToString([]byte(pdf.String())),
	})
	if err != nil || image != nil || !strings.Contains(document, text) {
		t.Fatalf("PDF attachment extraction = %q, %v, %v", document, image, err)
	}
}
