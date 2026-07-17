# Team spec schema

Generated from `schema/team.schema.json` — do not edit by hand.

Run `go run ./internal/schemadoc schema/team.schema.json docs/schema.md` to regenerate.

## `datadog_team_memberships`

Datadog team memberships. admins manage the team; members have read-only access.

- **type:** `object`
- **required:** true

## `display_name`

Team display name. Title Case; spaces and the lowercase word "and" are allowed.

- **type:** `string`
- **required:** true
- **pattern:** `^[A-Z][A-Za-z]*( (and|[A-Z][A-Za-z]*))*$`

## `display_name_comment`

Inline comment rendered after display_name in the tfvars file. Used for the team etymology blurb. Also used as the `description` frontmatter on the team's docs index page; render_team_docs_index and open_team_docs_pr require this field to be non-empty — an empty string will produce a docs_input_invalid error.

- **type:** `string`
- **required:** true

## `enable_google_project`

Enable a Google Cloud project for this team in the team's environment folder via pt-corpus. Default: false.

- **type:** `boolean`
- **required:** false

## `enable_opentofu_state_management`

Enable OpenTofu state management. Requires enable_workflows = true. When true, creates a GCS state storage bucket and grants the GitHub Actions service account Storage Object Admin and Cloud KMS CryptoKey Encrypter/Decrypter IAM roles.

- **type:** `boolean`
- **required:** false

## `enable_workflows`

Enable GitHub Actions CI/CD integration. When true, creates a GCP service account for GitHub Actions, Workload Identity Federation bindings (one per repository with enable_google_wif_service_account = true), and group memberships for console browse access, billing account viewer, and Artifact Registry write access.

- **type:** `boolean`
- **required:** false

## `github_child_teams_memberships`

GitHub child team memberships. The four standard teams (sandbox-approvers, non-production-approvers, production-approvers, repository-administrators) are always created; this block sets memberships and may add custom child teams.

- **type:** `object`
- **required:** false

## `github_parent_team_memberships`

GitHub team membership. maintainers have admin rights on the team; members have read access.

- **type:** `object`
- **required:** true

## `github_repositories`

GitHub repositories owned by this team. Key is the repository name. Each repository is provisioned with squash-only merges, a branch ruleset (signed commits, linear history, PR reviews), Datadog webhook, and standard repository files.

- **type:** `object`
- **required:** false

## `google_basic_groups_env_memberships`

Per-environment Google Cloud Identity basic groups: admin, reader, writer. Binds each role directly to environment folders — no team-folder binding, no inheritance. Allows finer-grained access control than team-level groups (e.g. write in sandbox/non-production, read-only in production).

- **type:** `object`
- **required:** true

## `google_browser_groups_memberships`

Google Cloud Identity group memberships scoped to deployment environments. Each present key (sandbox, non-production, production) configures the group membership for that environment. Omit an environment key if no group membership is needed for it.

- **type:** `object`
- **required:** false

## `google_project_creator_groups_memberships`

Google Cloud Identity group memberships scoped to deployment environments. Each present key (sandbox, non-production, production) configures the group membership for that environment. Omit an environment key if no group membership is needed for it.

- **type:** `object`
- **required:** false

## `google_project_enable_datadog`

Enable Datadog Google Cloud integration for the team project.

- **type:** `boolean`
- **required:** false

## `google_project_services`

Additional GCP API services to enable in the team project.

- **type:** `array`
- **required:** false

## `google_xpn_admin_groups_memberships`

Google Cloud Identity group memberships scoped to deployment environments. Each present key (sandbox, non-production, production) configures the group membership for that environment. Omit an environment key if no group membership is needed for it.

- **type:** `object`
- **required:** false

## `platform_managed_project`

Platform-managed project configuration. Presence of this block drives creation of a shared GCP project in pt-corpus that hosts GKE clusters or managed data services (Cloud SQL). The project is provisioned in the team's environment folder using the Arche modules. Omit entirely if the team needs neither.

- **type:** `object`
- **required:** false

## `team_key`

Team key. Must match the team_type prefix scheme: pt- (platform), st- (stream-aligned), ct- (complicated-subsystem), et- (enabling).

- **type:** `string`
- **required:** true
- **pattern:** `^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$`

## `team_type`

Team Topologies type. Must match the team_key prefix.

- **type:** `string`
- **required:** true
- **enum:** `platform-team`, `stream-aligned-team`, `complicated-subsystem-team`, `enabling-team`
