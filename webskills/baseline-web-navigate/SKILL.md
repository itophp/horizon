---
name: baseline-web-navigate
description: Navigate and inspect an explicitly connected Baseline application browser tab through its baseline-web MCP tools. Use for page navigation and workflow setup, not source code edits.
---

Use the baseline-web MCP connection installed from the application's WebMCP settings page.
Call `baseline_page_context` with `{}` to discover the current route and permitted destinations. Call `baseline_navigate` with `{ "path": "/accounts" }` (or another returned route). Its response acknowledges the navigation request, not completed rendering. Re-read context and wait for page-specific tools to appear before using them. `baseline_module_context` describes the active module and permitted routes; `baseline_current_page` provides the active route pattern and interaction counts. Global tools follow the connection, module tools follow the active module, and page tools follow the current page. The server sends a standard tool-list change notification; clients should refresh automatically. If the client keeps a stale catalog, reconnect its MCP connection. Registrations follow React page lifecycles.

If disconnected, ask the operator to start `pnpm webmcp`, sign in, open `/webmcp`, enter the terminal pairing key and connect. Run the client's one-click setup again after reconnecting or selecting a different tab, then restart the client. Never fetch or print the pairing key or application cookies. The local service is on port 18793 by default; it can be overridden by WEBMCP_PORT and must match the page setting.

Tools operate under the signed-in account and current department. A missing route or tool is not authorization to bypass it with an API request or arbitrary JavaScript. The application exposes navigation, account filtering/read-only details, organization/department/role list filters and projected diagnostics; it does not expose business write tools. The list filters accept `{ "query": "text" }`, update the visible search field and acknowledge the request; verify the rendered state after invoking them.
