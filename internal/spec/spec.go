// Package spec defines the typed Go representation of a pt-logos team.
//
// The shape mirrors schema/team.schema.json. The schema is the source of
// truth — when updating either, update both.
package spec

// Team is a single team's full configuration.
type Team struct {
	TeamKey                         string                           `json:"team_key"`
	DatadogTeamMemberships          DatadogTeamMemberships           `json:"datadog_team_memberships"`
	DisplayName                     string                           `json:"display_name"`
	DisplayNameComment              string                           `json:"display_name_comment,omitempty"`
	EnableGoogleProject             *bool                            `json:"enable_google_project,omitempty"`
	EnableOpenTofuStateManagement   *bool                            `json:"enable_opentofu_state_management,omitempty"`
	EnableWorkflows                 *bool                            `json:"enable_workflows,omitempty"`
	GitHubChildTeamsMemberships     map[string]GitHubMembership      `json:"github_child_teams_memberships,omitempty"`
	GitHubParentTeamMemberships     GitHubMembership                 `json:"github_parent_team_memberships"`
	GitHubRepositories              map[string]GitHubRepository      `json:"github_repositories,omitempty"`
	GitHubRepositoryLabels          map[string]GitHubRepositoryLabel `json:"github_repository_labels,omitempty"`
	GoogleBasicGroupsEnvMemberships GoogleBasicGroupsEnvMemberships  `json:"google_basic_groups_env_memberships"`
	GoogleBrowserGroups             *EnvScopedGoogleGroups           `json:"google_browser_groups_memberships,omitempty"`
	GoogleProjectCreatorGroups      *EnvScopedGoogleGroups           `json:"google_project_creator_groups_memberships,omitempty"`
	GoogleProjectEnableDatadog      *bool                            `json:"google_project_enable_datadog,omitempty"`
	GoogleProjectServices           []string                         `json:"google_project_services,omitempty"`
	GoogleXPNAdminGroups            *EnvScopedGoogleGroups           `json:"google_xpn_admin_groups_memberships,omitempty"`
	PlatformManagedProject          *PlatformManagedProject          `json:"platform_managed_project,omitempty"`
	TeamType                        string                           `json:"team_type"`
}

type DatadogTeamMemberships struct {
	Admins  []string `json:"admins"`
	Members []string `json:"members"`
}

type GitHubMembership struct {
	Maintainers []string `json:"maintainers"`
	Members     []string `json:"members"`
}

type GoogleGroup struct {
	Managers []string `json:"managers"`
	Members  []string `json:"members"`
	Owners   []string `json:"owners"`
}

type GoogleBasicGroupsEnvMemberships struct {
	Admin  EnvScopedGoogleGroups `json:"admin"`
	Reader EnvScopedGoogleGroups `json:"reader"`
	Writer EnvScopedGoogleGroups `json:"writer"`
}

type EnvScopedGoogleGroups struct {
	Sandbox       GoogleGroup `json:"sandbox"`
	NonProduction GoogleGroup `json:"non-production"`
	Production    GoogleGroup `json:"production"`
}

type GitHubRepository struct {
	Description                   string                       `json:"description"`
	EnableDatadogSecrets          *bool                        `json:"enable_datadog_secrets,omitempty"`
	EnableDatadogWebhook          *bool                        `json:"enable_datadog_webhook,omitempty"`
	EnableGoogleWIFServiceAccount *bool                        `json:"enable_google_wif_service_account,omitempty"`
	EnableRuleset                 *bool                        `json:"enable_ruleset,omitempty"`
	Environments                  map[string]GitHubEnvironment `json:"environments,omitempty"`
	Pages                         *GitHubPages                 `json:"pages,omitempty"`
	Topics                        []string                     `json:"topics"`
}

type GitHubRepositoryLabel struct {
	Color       string `json:"color"`
	Description string `json:"description"`
}

type GitHubEnvironment struct {
	DeploymentBranchPolicy *DeploymentBranchPolicy `json:"deployment_branch_policy,omitempty"`
	Name                   string                  `json:"name"`
	Reviewers              EnvironmentReviewers    `json:"reviewers"`
}

type DeploymentBranchPolicy struct {
	CustomBranchPolicies bool `json:"custom_branch_policies"`
	ProtectedBranches    bool `json:"protected_branches"`
}

type EnvironmentReviewers struct {
	Teams []string `json:"teams"`
}

type GitHubPages struct {
	BuildType string             `json:"build_type,omitempty"`
	CName     string             `json:"cname,omitempty"`
	Source    *GitHubPagesSource `json:"source,omitempty"`
}

type GitHubPagesSource struct {
	Branch string `json:"branch"`
	Path   string `json:"path,omitempty"`
}

type PlatformManagedProject struct {
	CloudSQL         *CloudSQL         `json:"cloud_sql,omitempty"`
	EnableDatadog    *bool             `json:"enable_datadog,omitempty"`
	KubernetesEngine *KubernetesEngine `json:"kubernetes_engine,omitempty"`
}

type CloudSQL struct {
	DatabaseVersion string   `json:"database_version,omitempty"`
	MachineTier     string   `json:"machine_tier,omitempty"`
	Regions         []string `json:"regions"`
}

type KubernetesEngine struct {
	ArtifactRegistryGroups *ArtifactRegistryGroups `json:"artifact_registry_groups_memberships,omitempty"`
	DNSSubdomain           string                  `json:"dns_subdomain,omitempty"`
	EnableDatadogAPM       *bool                   `json:"enable_datadog_apm,omitempty"`
	Locations              map[string]GKELocation  `json:"locations"`
	Namespaces             map[string]GKENamespace `json:"namespaces,omitempty"`
}

type GKENamespace struct {
	IstioInjection string              `json:"istio_injection,omitempty"`
	Routes         map[string]GKERoute `json:"routes,omitempty"`
}

type GKERoute struct {
	Path    string `json:"path,omitempty"`
	Port    int    `json:"port"`
	Service string `json:"service"`
}

type ArtifactRegistryGroups struct {
	Readers GoogleGroup `json:"readers"`
	Writers GoogleGroup `json:"writers"`
}

type GKELocation struct {
	EnableGKEHubHost *bool               `json:"enable_gke_hub_host,omitempty"`
	NodePools        map[string]NodePool `json:"node_pools"`
	Subnet           Subnet              `json:"subnet"`
}

type NodePool struct {
	MachineType  string `json:"machine_type"`
	MaxNodeCount int    `json:"max_node_count"`
	MinNodeCount int    `json:"min_node_count"`
}

type Subnet struct {
	IPCidrRange         string `json:"ip_cidr_range"`
	MasterIPv4CidrBlock string `json:"master_ipv4_cidr_block"`
	PodIPCidrRange      string `json:"pod_ip_cidr_range"`
	ServicesIPCidrRange string `json:"services_ip_cidr_range"`
}
