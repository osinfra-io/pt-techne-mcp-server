// Package render produces canonical tfvars bytes from a typed Team spec.
//
// Output rules (canonical formatting):
//
//   - Fields are emitted in a fixed order (see Render). Maps with named keys
//     are sorted alphabetically.
//   - One blank line between top-level field blocks. No blank lines inside
//     a leaf block.
//   - Adjacent simple scalar assignments share = alignment (computed per run).
//   - No trailing commas in lists. No stray inline comments — except the
//     optional display_name etymology comment.
//   - Topics and other string lists render one per line, sorted? No — topics
//     order is preserved as authored (it conveys meaning); only object map
//     keys are sorted.
//   - github_repositories keys and platform_managed_project.kubernetes_engine
//     locations keys are quoted (HCL string keys); all other map keys are
//     bare identifiers.
//
// Output is deterministic: same input -> byte-identical output, every run.
package render

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// Render renders a Team to canonical tfvars bytes.
func Render(t *spec.Team) ([]byte, error) {
	if t.TeamKey == "" {
		return nil, fmt.Errorf("render: team_key is required")
	}
	var b bytes.Buffer
	b.WriteString("teams = {\n")
	b.WriteString("  ")
	b.WriteString(t.TeamKey)
	b.WriteString(" = {\n")
	w := &writer{indent: 4}
	emitTeamBody(w, t)
	b.Write(w.buf.Bytes())
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.Bytes(), nil
}

// writer accumulates bytes with a base indentation level.
type writer struct {
	buf    bytes.Buffer
	indent int
}

func (w *writer) line(s string)  { w.write(strings.Repeat(" ", w.indent) + s + "\n") }
func (w *writer) blank()         { w.write("\n") }
func (w *writer) write(s string) { w.buf.WriteString(s) }

// emitTeamBody emits the body of the team block (no enclosing braces).
func emitTeamBody(w *writer, t *spec.Team) {
	first := true
	emit := func(fn func()) {
		if !first {
			w.blank()
		}
		first = false
		fn()
	}

	emit(func() { emitDatadog(w, t.DatadogTeamMemberships) })
	emit(func() { emitDisplayName(w, t.DisplayName, t.DisplayNameComment) })

	// Adjacent boolean group: enable_opentofu_state_management, enable_workflows
	if t.EnableOpenTofuStateManagement != nil || t.EnableWorkflows != nil {
		emit(func() { emitEnableGroup(w, t) })
	}

	if len(t.GitHubChildTeamsMemberships) > 0 {
		emit(func() { emitGitHubChildTeams(w, t.GitHubChildTeamsMemberships) })
	}
	emit(func() { emitGitHubMembership(w, "github_parent_team_memberships", t.GitHubParentTeamMemberships) })

	if len(t.GitHubRepositories) > 0 {
		emit(func() { emitGitHubRepositories(w, t.GitHubRepositories) })
	}

	emit(func() { emitGoogleBasicGroups(w, t.GoogleBasicGroupsMemberships) })

	if t.GoogleBrowserGroups != nil {
		emit(func() { emitEnvScopedGroups(w, "google_browser_groups_memberships", *t.GoogleBrowserGroups) })
	}
	if t.GoogleProjectCreatorGroups != nil {
		emit(func() {
			emitEnvScopedGroups(w, "google_project_creator_groups_memberships", *t.GoogleProjectCreatorGroups)
		})
	}
	if t.GoogleXPNAdminGroups != nil {
		emit(func() { emitEnvScopedGroups(w, "google_xpn_admin_groups_memberships", *t.GoogleXPNAdminGroups) })
	}

	if t.PlatformManagedProject != nil {
		emit(func() { emitPlatformManagedProject(w, *t.PlatformManagedProject) })
	}

	// Final adjacent group: project-level toggles + services. Aligned together.
	if t.EnableGoogleProject != nil || t.GoogleProjectEnableDatadog != nil || len(t.GoogleProjectServices) > 0 {
		emit(func() { emitProjectToggles(w, t) })
	}

	emit(func() { w.line("team_type = " + quote(t.TeamType)) })
}

func emitDatadog(w *writer, d spec.DatadogTeamMemberships) {
	w.line("datadog_team_memberships = {")
	w2 := nested(w)
	w2.aligned([][2]string{
		{"admins", emitStringList(d.Admins)},
		{"members", emitStringList(d.Members)},
	})
	w.merge(w2)
	w.line("}")
}

func emitDisplayName(w *writer, name, comment string) {
	line := "display_name = " + quote(name)
	if comment != "" {
		line += " # " + comment
	}
	w.line(line)
}

func emitEnableGroup(w *writer, t *spec.Team) {
	var rows [][2]string
	if t.EnableOpenTofuStateManagement != nil {
		rows = append(rows, [2]string{"enable_opentofu_state_management", boolStr(*t.EnableOpenTofuStateManagement)})
	}
	if t.EnableWorkflows != nil {
		rows = append(rows, [2]string{"enable_workflows", boolStr(*t.EnableWorkflows)})
	}
	w.alignedTop(rows)
}

func emitProjectToggles(w *writer, t *spec.Team) {
	var rows [][2]string
	if t.EnableGoogleProject != nil {
		rows = append(rows, [2]string{"enable_google_project", boolStr(*t.EnableGoogleProject)})
	}
	if t.GoogleProjectEnableDatadog != nil {
		rows = append(rows, [2]string{"google_project_enable_datadog", boolStr(*t.GoogleProjectEnableDatadog)})
	}
	if len(t.GoogleProjectServices) > 0 {
		// google_project_services renders as a multi-line list — emit separately, no alignment.
		w.alignedTop(rows)
		if len(rows) > 0 {
			w.blank()
		}
		emitMultilineStringList(w, "google_project_services", t.GoogleProjectServices)
		return
	}
	w.alignedTop(rows)
}

func emitGitHubChildTeams(w *writer, m map[string]spec.GitHubMembership) {
	w.line("github_child_teams_memberships = {")
	w.indent += 2
	for _, k := range sortedKeys(m) {
		w.line(k + " = {")
		w.indent += 2
		emitMembershipBody(w, m[k])
		w.indent -= 2
		w.line("}")
	}
	w.indent -= 2
	w.line("}")
}

func emitGitHubMembership(w *writer, name string, m spec.GitHubMembership) {
	nestedBlock(w, name+" = {", func(inner *writer) {
		emitMembershipBody(inner, m)
	})
}

func emitMembershipBody(w *writer, m spec.GitHubMembership) {
	w.aligned([][2]string{
		{"maintainers", emitStringList(m.Maintainers)},
		{"members", emitStringList(m.Members)},
	})
}

func emitGitHubRepositories(w *writer, repos map[string]spec.GitHubRepository) {
	w.line("github_repositories = {")
	keys := sortedKeys(repos)
	for i, k := range keys {
		if i > 0 {
			w.blank()
		}
		w.indent += 2
		w.line(quote(k) + " = {")
		w.indent += 2
		emitRepoBody(w, repos[k])
		w.indent -= 2
		w.line("}")
		w.indent -= 2
	}
	w.line("}")
}

func emitRepoBody(w *writer, r spec.GitHubRepository) {
	// description and any boolean toggles render as one aligned group.
	rows := [][2]string{{"description", quote(r.Description)}}
	if r.EnableDatadogSecrets != nil {
		rows = append(rows, [2]string{"enable_datadog_secrets", boolStr(*r.EnableDatadogSecrets)})
	}
	if r.EnableDatadogWebhook != nil {
		rows = append(rows, [2]string{"enable_datadog_webhook", boolStr(*r.EnableDatadogWebhook)})
	}
	if r.EnableGoogleWIFServiceAccount != nil {
		rows = append(rows, [2]string{"enable_google_wif_service_account", boolStr(*r.EnableGoogleWIFServiceAccount)})
	}
	if r.EnableRuleset != nil {
		rows = append(rows, [2]string{"enable_ruleset", boolStr(*r.EnableRuleset)})
	}
	w.alignedTop(rows)

	if len(r.Environments) > 0 {
		w.blank()
		emitEnvironments(w, r.Environments)
	}

	if r.Pages != nil {
		w.blank()
		emitPages(w, *r.Pages)
	}

	w.blank()
	emitMultilineStringList(w, "topics", r.Topics)
}

func emitEnvironments(w *writer, envs map[string]spec.GitHubEnvironment) {
	w.line("environments = {")
	w.indent += 2
	for _, k := range sortedKeys(envs) {
		w.line(k + " = {")
		w.indent += 2
		emitEnvironmentBody(w, envs[k])
		w.indent -= 2
		w.line("}")
	}
	w.indent -= 2
	w.line("}")
}

func emitEnvironmentBody(w *writer, e spec.GitHubEnvironment) {
	if e.DeploymentBranchPolicy != nil {
		w.line("deployment_branch_policy = {")
		w.indent += 2
		w.alignedTop([][2]string{
			{"custom_branch_policies", boolStr(e.DeploymentBranchPolicy.CustomBranchPolicies)},
			{"protected_branches", boolStr(e.DeploymentBranchPolicy.ProtectedBranches)},
		})
		w.indent -= 2
		w.line("}")
	}
	w.line("name = " + quote(e.Name))
	w.line("reviewers = {")
	w.indent += 2
	w.line("teams = " + emitStringList(e.Reviewers.Teams))
	w.indent -= 2
	w.line("}")
}

func emitPages(w *writer, p spec.GitHubPages) {
	w.line("pages = {")
	w.indent += 2
	var rows [][2]string
	if p.BuildType != "" {
		rows = append(rows, [2]string{"build_type", quote(p.BuildType)})
	}
	if p.CName != "" {
		rows = append(rows, [2]string{"cname", quote(p.CName)})
	}
	if len(rows) > 0 {
		w.alignedTop(rows)
	}
	if p.Source != nil {
		if len(rows) > 0 {
			w.blank()
		}
		w.line("source = {")
		w.indent += 2
		var srows [][2]string
		srows = append(srows, [2]string{"branch", quote(p.Source.Branch)})
		if p.Source.Path != "" {
			srows = append(srows, [2]string{"path", quote(p.Source.Path)})
		}
		w.alignedTop(srows)
		w.indent -= 2
		w.line("}")
	}
	w.indent -= 2
	w.line("}")
}

func emitGoogleBasicGroups(w *writer, g spec.GoogleBasicGroupsMemberships) {
	w.line("google_basic_groups_memberships = {")
	w.indent += 2
	emitGoogleGroupNamed(w, "admin", g.Admin)
	emitGoogleGroupNamed(w, "reader", g.Reader)
	emitGoogleGroupNamed(w, "writer", g.Writer)
	w.indent -= 2
	w.line("}")
}

func emitEnvScopedGroups(w *writer, name string, g spec.EnvScopedGoogleGroups) {
	w.line(name + " = {")
	w.indent += 2
	// Order: non-production, production, sandbox (alphabetical).
	emitGoogleGroupNamed(w, "non-production", g.NonProduction)
	emitGoogleGroupNamed(w, "production", g.Production)
	emitGoogleGroupNamed(w, "sandbox", g.Sandbox)
	w.indent -= 2
	w.line("}")
}

func emitGoogleGroupNamed(w *writer, name string, g spec.GoogleGroup) {
	w.line(name + " = {")
	w.indent += 2
	w.aligned([][2]string{
		{"managers", emitStringList(g.Managers)},
		{"members", emitStringList(g.Members)},
		{"owners", emitStringList(g.Owners)},
	})
	w.indent -= 2
	w.line("}")
}

func emitPlatformManagedProject(w *writer, p spec.PlatformManagedProject) {
	w.line("platform_managed_project = {")
	w.indent += 2
	first := true
	em := func(fn func()) {
		if !first {
			w.blank()
		}
		first = false
		fn()
	}
	if p.CloudSQL != nil {
		em(func() { emitCloudSQL(w, *p.CloudSQL) })
	}
	if p.EnableDatadog != nil {
		em(func() { w.line("enable_datadog = " + boolStr(*p.EnableDatadog)) })
	}
	if p.KubernetesEngine != nil {
		em(func() { emitKubernetesEngine(w, *p.KubernetesEngine) })
	}
	w.indent -= 2
	w.line("}")
}

func emitCloudSQL(w *writer, c spec.CloudSQL) {
	w.line("cloud_sql = {")
	w.indent += 2
	var rows [][2]string
	if c.DatabaseVersion != "" {
		rows = append(rows, [2]string{"database_version", quote(c.DatabaseVersion)})
	}
	if c.MachineTier != "" {
		rows = append(rows, [2]string{"machine_tier", quote(c.MachineTier)})
	}
	rows = append(rows, [2]string{"regions", emitStringList(c.Regions)})
	w.alignedTop(rows)
	w.indent -= 2
	w.line("}")
}

func emitKubernetesEngine(w *writer, k spec.KubernetesEngine) {
	w.line("kubernetes_engine = {")
	w.indent += 2
	first := true
	em := func(fn func()) {
		if !first {
			w.blank()
		}
		first = false
		fn()
	}
	if k.ArtifactRegistryGroups != nil {
		em(func() {
			w.line("artifact_registry_groups_memberships = {")
			w.indent += 2
			emitGoogleGroupNamed(w, "readers", k.ArtifactRegistryGroups.Readers)
			emitGoogleGroupNamed(w, "writers", k.ArtifactRegistryGroups.Writers)
			w.indent -= 2
			w.line("}")
		})
	}

	// dns_subdomain + enable_datadog_apm aligned
	var rows [][2]string
	if k.DNSSubdomain != "" {
		rows = append(rows, [2]string{"dns_subdomain", quote(k.DNSSubdomain)})
	}
	if k.EnableDatadogAPM != nil {
		rows = append(rows, [2]string{"enable_datadog_apm", boolStr(*k.EnableDatadogAPM)})
	}
	if len(rows) > 0 {
		em(func() { w.alignedTop(rows) })
	}

	em(func() { emitGKELocations(w, k.Locations) })
	w.indent -= 2
	w.line("}")
}

func emitGKELocations(w *writer, locs map[string]spec.GKELocation) {
	w.line("locations = {")
	keys := sortedKeys(locs)
	for i, k := range keys {
		if i > 0 {
			w.blank()
		}
		w.indent += 2
		w.line(quote(k) + " = {")
		w.indent += 2
		emitGKELocationBody(w, locs[k])
		w.indent -= 2
		w.line("}")
		w.indent -= 2
	}
	w.line("}")
}

func emitGKELocationBody(w *writer, l spec.GKELocation) {
	if l.EnableGKEHubHost != nil {
		w.line("enable_gke_hub_host = " + boolStr(*l.EnableGKEHubHost))
	}
	w.line("node_pools = {")
	w.indent += 2
	npKeys := sortedKeys(l.NodePools)
	for i, k := range npKeys {
		if i > 0 {
			w.blank()
		}
		np := l.NodePools[k]
		w.line(k + " = {")
		w.indent += 2
		w.alignedTop([][2]string{
			{"machine_type", quote(np.MachineType)},
			{"max_node_count", fmt.Sprintf("%d", np.MaxNodeCount)},
			{"min_node_count", fmt.Sprintf("%d", np.MinNodeCount)},
		})
		w.indent -= 2
		w.line("}")
	}
	w.indent -= 2
	w.line("}")
	w.line("subnet = {")
	w.indent += 2
	w.alignedTop([][2]string{
		{"ip_cidr_range", quote(l.Subnet.IPCidrRange)},
		{"master_ipv4_cidr_block", quote(l.Subnet.MasterIPv4CidrBlock)},
		{"pod_ip_cidr_range", quote(l.Subnet.PodIPCidrRange)},
		{"services_ip_cidr_range", quote(l.Subnet.ServicesIPCidrRange)},
	})
	w.indent -= 2
	w.line("}")
}

// nested returns a fresh writer at +2 indent for collecting block bodies before
// merge. Currently only used by emitDatadog; most callers just bump indent
// directly. Kept for clarity in the few call sites that benefit.
func nested(w *writer) *writer {
	return &writer{indent: w.indent + 2}
}

func (w *writer) merge(other *writer) { w.buf.Write(other.buf.Bytes()) }

// emitMultilineStringList emits `name = [\n  "v1",\n  "v2"\n]` with the list
// indented one level inside the current block. The terminating bracket sits
// at the parent indent — matching the on-disk style for `topics` and
// `google_project_services`.
func emitMultilineStringList(w *writer, name string, vs []string) {
	w.line(name + " = [")
	for _, v := range vs {
		w.write(strings.Repeat(" ", w.indent+2) + quote(v) + ",\n")
	}
	// Drop trailing comma from last element by rewriting last newline.
	b := w.buf.Bytes()
	// Find last ",\n" and strip the comma.
	if n := len(b); n >= 2 && b[n-2] == ',' && b[n-1] == '\n' {
		w.buf.Truncate(n - 2)
		w.buf.WriteByte('\n')
	}
	w.line("]")
}

// nestedBlock emits `header { body }` with body indented +2.
func nestedBlock(w *writer, header string, body func(*writer)) {
	w.line(header)
	w.indent += 2
	body(w)
	w.indent -= 2
	w.line("}")
}

// aligned renders rows as `key = value` lines with `=` aligned across rows.
// Used inside an already-indented block — no extra indent applied here beyond
// the writer's current setting.
func (w *writer) aligned(rows [][2]string) { w.alignedTop(rows) }

func (w *writer) alignedTop(rows [][2]string) {
	max := 0
	for _, r := range rows {
		if len(r[0]) > max {
			max = len(r[0])
		}
	}
	for _, r := range rows {
		pad := strings.Repeat(" ", max-len(r[0]))
		w.line(r[0] + pad + " = " + r[1])
	}
}

// emitStringList renders a string list as a single-line array, e.g.
// `["a", "b"]`. Used for inline lists like maintainers/members/teams/admins.
func emitStringList(vs []string) string {
	if len(vs) == 0 {
		return "[]"
	}
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = quote(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// quote renders an HCL string literal. We assume no control characters; the
// inputs are validated to be plain strings (emails, identifiers, descriptions).
// Backslashes and double-quotes are escaped.
func quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// sortedKeys returns the keys of a map[string]V sorted ascending.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
