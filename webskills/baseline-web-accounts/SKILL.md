---
name: baseline-web-accounts
description: Filter the connected Baseline account page and open authorized read-only account details using baseline-web MCP. Use for account page inspection, not account changes or bulk data extraction.
---

Navigate to `/accounts` with `baseline_navigate`, verify the current route, then wait for the client to refresh its tool list after the server notification. Page-specific tools only exist while the page is mounted. Wait for page loading to finish; a tool failure during loading is not evidence of an empty result.

- `baseline_accounts_filter`: `{ "query": "alice", "status": "active" }`. Query is optional (default empty, maximum 100 characters); status is `all`, `active` or `inactive` (default `all`). Replaces the page's text/status filter and returns only the matching count. It does not return account records.
- `baseline_accounts_details`: `{ "id": 42 }`. ID must be a positive integer already visible in the authorized page data. Opens the existing read-only detail dialog; does not return the record or submit changes.

Only use an account ID supplied by the user or already obtained through an authorized interface. Do not enumerate IDs. An unavailable account, stale session or missing permission must be handled as a failure, never as permission to use a broader query. Browser tools can inspect the rendered dialog when authorized, but must not retrieve hidden fields, passwords or credentials. Editing, deleting, department/permission changes and submissions are outside these MCP tools.
