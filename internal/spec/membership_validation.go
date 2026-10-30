package spec

func validateMembershipRequirements(spec any) []ValidationError {
	root, ok := spec.(map[string]any)
	if !ok {
		return nil
	}

	teamKey, _ := root["team_key"].(string)
	groups, _ := objectAt(root, "authentik_groups")
	var errs []ValidationError

	for _, key := range []string{"agentgateway-admins", "authentik-admins"} {
		if teamKey == "pt-pneuma" {
			if _, ok := groups[key]; !ok {
				errs = append(errs, ValidationError{
					Path:    "/authentik_groups/" + pointerPart(key),
					Message: "pt-pneuma must declare this Authentik group",
				})
			}
		} else if _, ok := groups[key]; ok {
			errs = append(errs, ValidationError{
				Path:    "/authentik_groups/" + pointerPart(key),
				Message: "this Authentik group is reserved for pt-pneuma",
			})
		}
	}

	if group, ok := groups["authentik-admins"].(map[string]any); ok {
		members, _ := objectAt(group, "members")
		for _, environment := range []string{"sandbox", "non-production", "production"} {
			memberList := stringList(members[environment])
			if !containsString(memberList, "brett@osinfra.io") {
				errs = append(errs, ValidationError{
					Path:    "/authentik_groups/authentik-admins/members/" + pointerPart(environment),
					Message: "pt-pneuma Authentik Admins must include brett@osinfra.io in every environment",
				})
			}
		}
	}

	project, _ := objectAt(root, "platform_managed_project")
	kubernetesEngine, ok := objectAt(project, "kubernetes_engine")
	if !ok {
		return errs
	}
	locations, _ := objectAt(kubernetesEngine, "locations")
	if len(locations) == 0 {
		return errs
	}

	artifactRegistryGroups, ok := objectAt(kubernetesEngine, "artifact_registry_groups_memberships")
	if !ok {
		return append(errs, ValidationError{
			Path:    "/platform_managed_project/kubernetes_engine/artifact_registry_groups_memberships",
			Message: "Kubernetes teams must declare Artifact Registry readers and writers, each with at least one owner",
		})
	}
	for _, role := range []string{"readers", "writers"} {
		group, _ := objectAt(artifactRegistryGroups, role)
		owners := stringList(group["owners"])
		if len(owners) == 0 {
			errs = append(errs, ValidationError{
				Path:    "/platform_managed_project/kubernetes_engine/artifact_registry_groups_memberships/" + role + "/owners",
				Message: "Artifact Registry groups must have at least one owner",
			})
		}
	}
	return errs
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
