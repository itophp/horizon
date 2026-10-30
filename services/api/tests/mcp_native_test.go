//go:build integration

package tests

import (
	gen "baseline/services/api/gen/mcp_management"
	"bufio"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestMCPNativeClients(t *testing.T) {
	if os.Getenv("MCP_NATIVE_ACCEPTANCE") != "1" {
		t.Skip("native client acceptance is a separate target: make test-mcp-native")
	}
	if os.Getenv("WORKBUDDY_CLI") == "" {
		t.Fatal("WORKBUDDY_CLI must point to the installed WorkBuddy runtime CLI")
	}
	handler, _, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	response := owner.request(t, "POST", "/api/auth/mcp/tokens", map[string]interface{}{"name": "native acceptance", "password": "TestOnly!2026Secure", "days": 1, "departmentId": root.DepartmentID, "tools": []string{"context_get", "capabilities_list"}}, true)
	checkStatus(t, response, 201)
	created := decodeBody[gen.MCPTokenCreated](t, response)
	server := httptest.NewServer(handler)
	defer server.Close()
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "node", filepath.Join(repo, "scripts/mcp-business-native-check.mjs"))
	command.Dir = repo
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer command.Process.Kill()
	if err = json.NewEncoder(stdin).Encode(map[string]string{"url": server.URL + "/mcp", "token": created.Secret}); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("native client initialization failed")
	}
	var connected struct {
		Phase              string
		CodexToolCall      bool
		WorkbuddyConnected bool
	}
	if err = json.Unmarshal(scanner.Bytes(), &connected); err != nil {
		t.Fatal(err)
	}
	if connected.Phase != "connected" || !connected.CodexToolCall || !connected.WorkbuddyConnected {
		t.Fatalf("native clients not connected: %+v", connected)
	}
	checkStatus(t, owner.request(t, "DELETE", "/api/auth/mcp/tokens/"+strconv.FormatInt(created.Token.ID, 10), map[string]string{"password": "TestOnly!2026Secure"}, true), 204)
	if _, err = stdin.Write([]byte("revoked\n")); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() {
		t.Fatal("native revocation verification failed")
	}
	var revoked struct {
		Phase             string
		CodexRejected     bool
		WorkbuddyRejected bool
	}
	if err = json.Unmarshal(scanner.Bytes(), &revoked); err != nil {
		t.Fatal(err)
	}
	if revoked.Phase != "revoked" || !revoked.CodexRejected || !revoked.WorkbuddyRejected {
		t.Fatal("native revoked token still usable")
	}
	stdin.Close()
	if err = command.Wait(); err != nil {
		t.Fatal(err)
	}
}
