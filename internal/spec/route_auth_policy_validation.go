package spec

import "regexp"

func validateRouteAuthPolicies(spec any) []ValidationError {
	root, ok := spec.(map[string]any)
	if !ok {
		return nil
	}
	project, ok := objectAt(root, "platform_managed_project")
	if !ok {
		return nil
	}
	gke, ok := objectAt(project, "kubernetes_engine")
	if !ok {
		return nil
	}
	namespaces, ok := objectAt(gke, "namespaces")
	if !ok {
		return nil
	}

	var errs []ValidationError
	for namespaceName, rawNamespace := range namespaces {
		namespace, ok := rawNamespace.(map[string]any)
		if !ok {
			continue
		}
		policies, ok := objectAt(namespace, "route_auth_policies")
		if !ok {
			continue
		}
		path := "/platform_managed_project/kubernetes_engine/namespaces/" + pointerPart(namespaceName)
		errs = append(errs, validateNamespaceRouteAuthPolicies(path, namespace, policies)...)
	}
	return errs
}

func validateNamespaceRouteAuthPolicies(path string, namespace map[string]any, policies map[string]any) []ValidationError {
	var errs []ValidationError
	if namespace["istio_injection"] != "enabled" {
		errs = append(errs, ValidationError{
			Path:    path + "/route_auth_policies",
			Message: "route_auth_policies may only be declared when istio_injection is enabled",
		})
	}

	routes, _ := objectAt(namespace, "routes")
	for routeName, rawPolicy := range policies {
		policyPath := path + "/route_auth_policies/" + pointerPart(routeName)
		if !routeAuthPolicyKeyValid(routeName) {
			errs = append(errs, ValidationError{
				Path:    policyPath,
				Message: "route_auth_policies keys must be RFC 1123 labels",
			})
		}
		route, exists := routes[routeName]
		if !exists {
			errs = append(errs, ValidationError{
				Path:    policyPath,
				Message: "route_auth_policies keys must match an existing route name",
			})
		}

		policy, ok := rawPolicy.(map[string]any)
		if !ok {
			continue
		}
		enforced := true
		if v, ok := policy["enforced"].(bool); ok {
			enforced = v
		}
		requiredGroups := stringList(policy["required_groups"])
		requiredRoles := stringList(policy["required_roles"])
		publicPaths := stringList(policy["public_paths"])

		if enforced {
			if len(requiredGroups)+len(requiredRoles) == 0 {
				errs = append(errs, ValidationError{
					Path:    policyPath,
					Message: "enforced route_auth_policies must declare required_groups or required_roles",
				})
			}
		} else {
			for _, field := range []string{"public_paths", "required_groups", "required_roles"} {
				if _, declared := policy[field]; declared {
					errs = append(errs, ValidationError{
						Path:    policyPath + "/" + field,
						Message: "disabled route_auth_policies must not declare public_paths, required_groups, or required_roles",
					})
				}
			}
		}

		routePath := routePathPrefix(route)
		for i, publicPath := range publicPaths {
			if !pathUnderPrefix(publicPath, routePath) {
				errs = append(errs, ValidationError{
					Path:    policyPath + "/public_paths/" + intPath(i),
					Message: "public_paths must be under the route path prefix",
				})
			}
		}
	}
	return errs
}

func routeAuthPolicyKeyValid(routeName string) bool {
	ok, _ := regexp.MatchString(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`, routeName)
	return ok
}

func objectAt(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	out, ok := v.(map[string]any)
	return out, ok
}

func stringList(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func routePathPrefix(route any) string {
	routeMap, ok := route.(map[string]any)
	if !ok {
		return "/"
	}
	path, ok := routeMap["path"].(string)
	if !ok || path == "" {
		return "/"
	}
	return path
}

func pathUnderPrefix(path, prefix string) bool {
	if prefix == "/" {
		return path != "/"
	}
	return path == prefix || len(path) > len(prefix) && path[:len(prefix)+1] == prefix+"/"
}

func pointerPart(s string) string {
	var out []rune
	for _, r := range s {
		switch r {
		case '~':
			out = append(out, '~', '0')
		case '/':
			out = append(out, '~', '1')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

func intPath(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
