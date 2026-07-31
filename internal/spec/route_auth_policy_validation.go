package spec

import (
	"regexp"
	"unicode"
)

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
		mode := "browser"
		if v, ok := policy["mode"].(string); ok {
			mode = v
		}
		audiences := stringList(policy["audiences"])
		requiredGroups := stringList(policy["required_groups"])
		requiredRoles := stringList(policy["required_roles"])
		publicPaths := stringList(policy["public_paths"])

		errs = append(errs, validateRouteAuthPolicyList(policyPath, "audiences", audiences)...)
		errs = append(errs, validateRouteAuthPolicyList(policyPath, "public_paths", publicPaths)...)
		errs = append(errs, validateRouteAuthPolicyList(policyPath, "required_groups", requiredGroups)...)
		errs = append(errs, validateRouteAuthPolicyList(policyPath, "required_roles", requiredRoles)...)

		switch mode {
		case "public":
			if len(audiences)+len(publicPaths)+len(requiredGroups)+len(requiredRoles) > 0 {
				errs = append(errs, ValidationError{
					Path:    policyPath,
					Message: "public route_auth_policies must not declare audiences, public_paths, required_groups, or required_roles",
				})
			}
		case "browser":
			if len(audiences) > 0 {
				errs = append(errs, ValidationError{
					Path:    policyPath + "/audiences",
					Message: "browser route_auth_policies must not declare audiences",
				})
			}
			if len(requiredGroups)+len(requiredRoles) == 0 {
				errs = append(errs, ValidationError{
					Path:    policyPath,
					Message: "browser route_auth_policies must declare required_groups or required_roles",
				})
			}
		case "api-jwt":
			if len(audiences) == 0 {
				errs = append(errs, ValidationError{
					Path:    policyPath + "/audiences",
					Message: "api-jwt route_auth_policies must declare audiences",
				})
			}
		default:
			errs = append(errs, ValidationError{
				Path:    policyPath + "/mode",
				Message: "route_auth_policies mode must be one of public, browser, or api-jwt",
			})
		}

		routePath := routePathPrefix(route)
		for i, publicPath := range publicPaths {
			if !publicPathValid(publicPath) || !pathUnderPrefix(publicPath, routePath) {
				errs = append(errs, ValidationError{
					Path:    policyPath + "/public_paths/" + intPath(i),
					Message: "public_paths must start with /, contain no whitespace, stay under the route path prefix, and not be /",
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

func validateRouteAuthPolicyList(policyPath, field string, values []string) []ValidationError {
	var errs []ValidationError
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		itemPath := policyPath + "/" + field + "/" + intPath(i)
		if value == "" || containsWhitespace(value) {
			errs = append(errs, ValidationError{
				Path:    itemPath,
				Message: field + " entries must be non-empty strings with no whitespace",
			})
		}
		if _, ok := seen[value]; ok {
			errs = append(errs, ValidationError{
				Path:    itemPath,
				Message: field + " entries must be unique",
			})
			continue
		}
		seen[value] = struct{}{}
	}
	return errs
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
	for len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}

func pathUnderPrefix(path, prefix string) bool {
	if prefix == "/" {
		return path != "/"
	}
	return len(path) > len(prefix) && path[:len(prefix)+1] == prefix+"/"
}

func publicPathValid(path string) bool {
	return path != "/" && len(path) > 0 && path[0] == '/' && !containsWhitespace(path)
}

func containsWhitespace(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
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
