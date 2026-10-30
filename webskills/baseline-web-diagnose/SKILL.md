---
name: baseline-web-diagnose
description: Diagnose loading, request failures and form validation in a connected Baseline page using projected MCP diagnostics plus browser evidence. Use for reproducing page problems, not extracting application caches or credentials.
---

Start with `baseline_page_context` and `baseline_diagnostics`, both with `{}`. Diagnostics return the route, fetching query count, up to 20 stable request error codes/statuses, open dialog count and invalid field count. They intentionally omit query keys, cached responses, raw exceptions, field values and session information.

Use these signals to choose the next check:

- Pending requests: wait for observable completion or collect a bounded browser network trace. A pending count of zero does not prove success.
- HTTP 401/403: check the signed-in session/current department and server authorization. Do not bypass the denial.
- Invalid fields: inspect visible validation messages with browser accessibility tools. Do not dump all input values.
- Layout or interaction problems: capture a screenshot, inspect DOM/computed styles and reproduce with actual user clicks. A successful MCP action does not prove that a button is reachable or correctly wired.

For server investigation, correlate the affected request's safe request ID with authorized local logs; do not expose full headers, bodies, cookies or tokens. Report the reproduction steps, observed evidence, diagnosis and checks actually performed. MCP success alone is not an end-to-end test result. Refreshing or signing out disables the page connection; reconnect through `/mcp` (WebMCP tab) and run client setup again when needed.
