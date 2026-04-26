// Read-tool opError shapers.
//
// notConfigured and notFound are the two opError shapes the read tools
// share. They live next to the read tools rather than open_team_pr's
// errResult/apiError/opError because their messages reference read-tool
// behavior (GITHUB_TOKEN required, "team X not found"), not write-tool
// behavior.
package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// notConfigured returns the standard not_configured opError result for
// a read tool that was constructed with a nil github client (no
// GITHUB_TOKEN at server start).
func notConfigured(toolName string) *mcp.CallToolResult {
	return errResult(opError{
		Code:    "not_configured",
		Message: toolName + " requires GITHUB_TOKEN; see README Configuration",
	})
}

// notFound returns a not_found opError result. Used by get_team for
// unknown team_keys.
func notFound(message string) *mcp.CallToolResult {
	return errResult(opError{Code: "not_found", Message: message})
}
