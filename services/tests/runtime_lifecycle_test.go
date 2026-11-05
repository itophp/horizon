//go:build integration

package tests

import (
	gencap "baseline/services/api/gen/capability"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRuntimeLifecycleEnforcedAcrossEntries(t *testing.T) {
	handler, pool, _ := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	invoke := func(name string, input map[string]any) map[string]any {
		t.Helper()
		result := invokeCapability(t, root, name, map[string]any{"input": input, "idempotencyKey": fmt.Sprintf("lifecycle-%s-%v", name, input)})
		if result.Status != "succeeded" {
			t.Fatalf("%s: %+v", name, result)
		}
		return result.Data.(map[string]any)
	}
	catalog := invoke("application.runtime.installation.list", map[string]any{})
	if len(catalog["items"].([]any)) != 2 {
		t.Fatal(catalog)
	}
	project := invoke("project.create", map[string]any{"code": "LIFE", "name": "Lifecycle"})
	id := project["id"]
	invoke("application.runtime.installation.disable", map[string]any{"pluginId": "com.horizon.projects", "revision": 1})
	checkStatus(t, root.invoke(t, "project.get", map[string]any{"id": id}, true), 503)
	// A direct call and metadata resolution both deny a disabled source.
	denied := decodeBody[gencap.CapabilityResult](t, root.invoke(t, "application.runtime.resolve", map[string]any{"applicationId": "projects"}, true))
	if denied.Status != "failed" || denied.ErrorCode != "not_found" {
		t.Fatal(denied)
	}
	invoke("application.runtime.installation.enable", map[string]any{"pluginId": "com.horizon.projects", "revision": 2})
	invoke("project.get", map[string]any{"id": id})
	conflict := decodeBody[gencap.CapabilityResult](t, root.invoke(t, "application.runtime.installation.disable", map[string]any{"pluginId": "com.horizon.projects", "revision": 1}, true))
	if conflict.Status != "failed" || conflict.ErrorCode != "conflict" {
		t.Fatal(conflict)
	}
	invoke("application.runtime.application.disable", map[string]any{"pluginId": "com.horizon.projects", "applicationId": "projects", "revision": 3})
	checkStatus(t, root.invoke(t, "project.get", map[string]any{"id": id}, true), 503)
	invoke("application.runtime.application.enable", map[string]any{"pluginId": "com.horizon.projects", "applicationId": "projects", "revision": 4})
	invoke("application.runtime.installation.uninstall", map[string]any{"pluginId": "com.horizon.projects", "revision": 5})
	var count int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM projects WHERE tenant_id='tenant-a' AND code='LIFE'").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	invoke("application.runtime.installation.install", map[string]any{"pluginId": "com.horizon.projects", "version": "1.0.0"})
	checkStatus(t, root.invoke(t, "project.get", map[string]any{"id": id}, true), 503)
	invoke("application.runtime.installation.enable", map[string]any{"pluginId": "com.horizon.projects", "revision": 7})
	invoke("project.get", map[string]any{"id": id})
	definition := invoke("application.runtime.resolve", map[string]any{"applicationId": "projects"})
	if definition["schemaVersion"] != "2.1" {
		t.Fatal(definition["schemaVersion"])
	}
}

func TestRuntimeContextAndStaleUIReference(t *testing.T) {
	handler, _, _ := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	call := func(name string, input map[string]any) map[string]any {
		t.Helper()
		r := invokeCapability(t, client, name, map[string]any{"input": input, "idempotencyKey": fmt.Sprintf("context-%s-%d", name, time.Now().UnixNano())})
		if r.Status != "succeeded" {
			t.Fatal(r)
		}
		return r.Data.(map[string]any)
	}
	article := call("article.create", map[string]any{"title": "Safe title", "body": "Private article body"})
	definition := call("application.runtime.resolve", map[string]any{"applicationId": "articles"})
	context := call("application.runtime.context.resolve", map[string]any{"applicationId": "articles", "pageId": "article.detail", "recordId": article["id"], "metadataVersion": definition["metadataVersion"]})
	raw, err := json.Marshal(context)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Private article body") || !strings.Contains(string(raw), "Safe title") {
		t.Fatal(string(raw))
	}
	call("application.runtime.installation.disable", map[string]any{"pluginId": "builtin:articles", "revision": 1})
	call("application.runtime.installation.enable", map[string]any{"pluginId": "builtin:articles", "revision": 2})
	response := client.request(t, "POST", "/api/capabilities/article.update/invocations", map[string]any{"version": "1.0.0", "input": map[string]any{"id": article["id"], "revision": article["revision"], "title": "Stale save", "body": "Private article body"}, "idempotencyKey": "stale-save", "runtime": map[string]any{"applicationId": "articles", "pageId": "article.detail", "metadataVersion": definition["metadataVersion"], "installationRevision": 1}}, true)
	if response.Code == 200 {
		r := decodeBody[gencap.CapabilityResult](t, response)
		if r.Status == "succeeded" {
			t.Fatal("stale form write accepted", r)
		}
	}
	current := call("article.get", map[string]any{"id": article["id"]})
	if current["title"] != "Safe title" {
		t.Fatal(current)
	}
}
