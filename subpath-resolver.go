package resolve

import (
	"strings"
)

func NormalizeMapping(m any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}

	if str, ok := m.(string); ok {
		return map[string]any{".": str}
	}

	if arr, ok := m.([]string); ok {
		return map[string]any{".": arr}
	}

	if mMap, ok := m.(map[string]any); ok {
		return mMap
	}

	return make(map[string]any)
}

func findWildcardMatch(mapping map[string]any, input string) (string, string, bool) {
	var bestMatchKey string
	var bestMatchReplacement string
	var bestMatchLength int

	for key := range mapping {
		if strings.HasSuffix(key, "/*") {
			prefix := strings.TrimSuffix(key, "*")
			if strings.HasPrefix(input, prefix) {
				matchLength := len(prefix)
				if matchLength > bestMatchLength {
					bestMatchKey = key
					bestMatchReplacement = strings.TrimPrefix(input, prefix)
					bestMatchLength = matchLength
				}
			}
		} else if strings.Contains(key, "*") {
			parts := strings.SplitN(key, "*", 2)
			if len(parts) != 2 {
				continue
			}
			before, after := parts[0], parts[1]
			if strings.HasPrefix(input, before) && strings.HasSuffix(input, after) {
				matchLength := len(before) + len(after)
				if matchLength > bestMatchLength {
					bestMatchKey = key
					bestMatchReplacement = strings.TrimPrefix(
						strings.TrimSuffix(input, after),
						before,
					)
					bestMatchLength = matchLength
				}
			}
		} else if strings.HasSuffix(key, "/") && strings.HasPrefix(input, key) {
			matchLength := len(key)
			if matchLength > bestMatchLength {
				bestMatchKey = key
				bestMatchReplacement = strings.TrimPrefix(input, key)
				bestMatchLength = matchLength
			}
		}
	}

	return bestMatchKey, bestMatchReplacement, bestMatchKey != ""
}

func resolveMappingValue(value any, conditions []string) []string {
	if value == nil {
		return nil
	}

	if str, ok := value.(string); ok {
		return []string{str}
	}

	if arr, ok := value.([]string); ok {
		var result []string
		for _, v := range arr {
			if resolved := resolveMappingValue(v, conditions); resolved != nil {
				result = append(result, resolved...)
			}
		}
		return result
	}

	if mMap, ok := value.(map[string]any); ok {
		for _, condition := range conditions {
			if subValue, exists := mMap[condition]; exists {
				return resolveMappingValue(subValue, conditions)
			}
		}
	}

	return nil
}

func resolveMapping(mapping map[string]any, conditions []string, input string) []string {
	var wildcardMatchKey string
	var wildcardReplacement string
	var hasWildcardMatch bool

	value, ok := mapping[input]
	if !ok {
		wildcardMatchKey, wildcardReplacement, hasWildcardMatch = findWildcardMatch(mapping, input)
		if hasWildcardMatch {
			if _, ok = mapping[wildcardMatchKey]; !ok {
				return nil
			}
		} else {
			return nil
		}
	}

	resolved := resolveMappingValue(value, conditions)
	if resolved == nil {
		return nil
	}

	if !hasWildcardMatch {
		return resolved
	}

	result := make([]string, len(resolved))
	for i, r := range resolved {
		if strings.Contains(r, "*") {
			parts := strings.SplitN(r, "*", 2)
			replacement := strings.TrimPrefix(wildcardReplacement, "/")
			result[i] = parts[0] + replacement + parts[1]
		} else if strings.HasSuffix(r, "/") {
			replacement := strings.TrimPrefix(wildcardReplacement, "/")
			result[i] = r + replacement
		} else {
			result[i] = r
		}
	}

	return result
}

const subpathPrefix = "./"

func normalizeEntry(entry string) string {
	if entry == "" || entry == "." || entry == subpathPrefix {
		return "."
	}
	if strings.HasPrefix(entry, subpathPrefix) {
		return entry
	}
	return subpathPrefix + entry
}

type SubpathResolver struct {
	Conditions []string
	Exports    map[string]any
	Imports    map[string]any
}

type SubpathResolverConfig struct {
	Exports    any
	Imports    map[string]any
	Conditions []string
}

func NewSubpathResolver(config SubpathResolverConfig) *SubpathResolver {
	conditions := config.Conditions
	if conditions == nil {
		conditions = []string{"default"}
	}

	return &SubpathResolver{
		Conditions: conditions,
		Exports:    NormalizeMapping(config.Exports),
		Imports:    config.Imports,
	}
}

func (r *SubpathResolver) ResolveExports(entry string) []string {
	return resolveMapping(r.Exports, r.Conditions, normalizeEntry(entry))
}

func (r *SubpathResolver) ResolveImports(entry string) []string {
	if r.Imports == nil {
		return nil
	}
	return resolveMapping(r.Imports, r.Conditions, entry)
}
