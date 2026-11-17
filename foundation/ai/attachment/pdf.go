package attachment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"horizon/foundation/kernel/capability/contracts"
)

const PDFUnavailable = "ai_pdf_unavailable"

func pdfText(ctx context.Context, raw []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pdftotext", "-enc", "UTF-8", "-", "-")
	cmd.Stdin = bytes.NewReader(raw)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", contracts.Problem(PDFUnavailable)
		}
		return "", err
	}
	result, readErr := io.ReadAll(io.LimitReader(stdout, maxTextBytes+1))
	if readErr != nil || len(result) > maxTextBytes {
		cancel()
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	if readErr != nil || len(result) > maxTextBytes {
		return "", contracts.Problem(Invalid)
	}
	return strings.ReplaceAll(string(result), "\f", "\n"), nil
}
