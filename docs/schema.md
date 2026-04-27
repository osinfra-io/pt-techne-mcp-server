# Team spec schema

Generated from `schema/team.schema.json` — do not edit by hand.

Run `make schema-docs` to regenerate.

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

Optional inline comment rendered after display_name. Used for the team etymology blurb. Also used as the `description` frontmatter on the team's docs index page (rendered by pt-techne-mcp-server/render_team_docs_index); that tool requires this field.

- **type:** `string`
- **required:** false

## `enable_google_project`

Enable a Google Cloud project for this team.

- **type:** `boolean`
- **required:** false

## `enable_opentofu_state_management`

Enable OpenTofu state management. Requires enable_workflows = true.

- **type:** `boolean`
- **required:** false

## `enable_workflows`

Enable GitHub Actions service account, workload identity federation, and group memberships.

- **type:** `boolean`
- **required:** false

## `github_child_teams_memberships`

GitHub child team memberships. The four standard teams (sandbox-approvers, non-production-approvers, production-approvers, repository-administrators) are always created; this block sets memberships and may add custom child teams.

- **type:** `object`
- **required:** false

## `github_parent_team_memberships`

GitHub parent team memberships. All team members should be listed here. Use GitHub usernames (NOT email addresses).

- **type:** `any`
- **required:** true

## `github_repositories`

GitHub repositories owned by this team.

- **type:** `object`
- **required:** false

## `google_basic_groups_memberships`

Google Cloud Identity basic groups: admin, reader, writer.

- **type:** `object`
- **required:** true

## `google_browser_groups_memberships`

pt-corpus only. Per-environment console browse access for pt-corpus service accounts.

- **type:** `any`
- **required:** false

## `google_project_creator_groups_memberships`

pt-corpus only. Per-environment project creator access for pt-corpus service accounts.

- **type:** `any`
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

pt-corpus only. Per-environment shared VPC (XPN) admin access for pt-corpus service accounts.

- **type:** `any`
- **required:** false

## `platform_managed_project`

Platform-managed project configuration. Drives creation of GKE clusters and managed data services in pt-corpus.

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

