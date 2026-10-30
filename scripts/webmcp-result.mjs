import { CallToolResultSchema } from "@modelcontextprotocol/sdk/types.js";

// WebMCP returns DOMString; next-sdk wraps it as MCP text without decoding.
// Decode only an exact MCP result envelope, preserving ordinary tool text.
export function normalizeWebMcpResult(result) {
  if (result.content?.length !== 1 || result.content[0].type !== "text")
    return result;
  let decoded;
  try {
    decoded = JSON.parse(result.content[0].text);
  } catch {
    return result;
  }
  if (
    !decoded ||
    typeof decoded !== "object" ||
    !Array.isArray(decoded.content)
  )
    return result;
  const parsed = CallToolResultSchema.safeParse(decoded);
  return parsed.success ? parsed.data : result;
}
