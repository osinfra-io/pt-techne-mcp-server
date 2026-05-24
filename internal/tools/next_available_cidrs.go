// MCP tool: next_available_cidrs.
//
// Computes the next N unallocated GKE subnet CIDR slots by scanning all
// teams in pt-logos@main. Eliminates the need for agents to perform
// multi-call CIDR arithmetic manually.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/cidr"
	gh "github.com/osinfra-io/pt-techne-mcp-server/internal/github"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// NextAvailableCidrsInput is the input for next_available_cidrs.
type NextAvailableCidrsInput struct {
	Count int `json:"count" jsonschema:"number of CIDR slots to return (minimum 1)"`
}

// NextAvailableCidrsOutput is the structured result of next_available_cidrs.
type NextAvailableCidrsOutput struct {
	Slots []cidr.Slot `json:"slots"`
}

// NextAvailableCidrs registers the next_available_cidrs tool.
// Requires GITHUB_TOKEN.
func NextAvailableCidrs(s *mcp.Server, v *spec.Validator, c gh.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "next_available_cidrs",
		Description: "Compute the next N unallocated GKE subnet CIDR slots by scanning all team specs in osinfra-io/pt-logos@main. Returns deterministic CIDR allocations for ip_cidr_range, pod_ip_cidr_range, services_ip_cidr_range, and master_ipv4_cidr_block. Requires GITHUB_TOKEN.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Next available CIDRs",
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in NextAvailableCidrsInput) (*mcp.CallToolResult, any, error) {
		const maxCount = 256
		if c == nil {
			return notConfigured("next_available_cidrs"), nil, nil
		}
		if in.Count < 1 || in.Count > maxCount {
			return errResult(opError{Code: "invalid_input", Message: "count must be between 1 and 256"}), nil, nil
		}
		ref, oe := resolveBaseRef(ctx, c)
		if oe != nil {
			return errResult(*oe), nil, nil
		}
		keys, oe := listTeamFiles(ctx, c, ref)
		if oe != nil {
			return errResult(*oe), nil, nil
		}
		teams, oe := fetchAllTeams(ctx, c, v, keys, ref)
		if oe != nil {
			return errResult(*oe), nil, nil
		}
		existing := cidr.CollectSubnets(teams)
		slots := cidr.NextAvailable(existing, in.Count)
		return nil, &NextAvailableCidrsOutput{Slots: slots}, nil
	})
}
