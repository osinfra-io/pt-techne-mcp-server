package spec

import "strings"

func validateAuthentikGroups(spec any) []ValidationError {
	root, ok := spec.(map[string]any)
	if !ok {
		return nil
	}
	groups, _ := objectAt(root, "authentik_groups")
	teamKey, _ := root["team_key"].(string)
	names := make(map[string]bool, len(groups))
	var errs []ValidationError
	for _, key := range sortedSpecKeys(groups) {
		group, _ := groups[key].(map[string]any)
		label, _ := group["label"].(string)
		name := teamKey + ": " + label
		if names[name] {
			errs = append(errs, ValidationError{
				Path:    "/authentik_groups/" + pointerPart(key) + "/label",
				Message: "Authentik group labels must be unique within the owning team",
			})
		}
		names[name] = true
	}
	project, _ := objectAt(root, "platform_managed_project")
	gke, _ := objectAt(project, "kubernetes_engine")
	namespaces, _ := objectAt(gke, "namespaces")
	for _, nsKey := range sortedSpecKeys(namespaces) {
		ns, _ := namespaces[nsKey].(map[string]any)
		policies, _ := objectAt(ns, "route_auth_policies")
		for _, routeKey := range sortedSpecKeys(policies) {
			policy, _ := policies[routeKey].(map[string]any)
			for _, field := range []string{"required_groups", "required_roles"} {
				for i, name := range stringList(policy[field]) {
					if !strings.Contains(name, ": ") {
						continue
					}
					if !names[name] {
						errs = append(errs, ValidationError{
							Path:    "/platform_managed_project/kubernetes_engine/namespaces/" + pointerPart(nsKey) + "/route_auth_policies/" + pointerPart(routeKey) + "/" + field + "/" + intPath(i),
							Message: "team-qualified Authentik groups must be declared by the owning team",
						})
					}
				}
			}
		}
	}
	return errs
}
