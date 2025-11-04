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

type match struct {
	key         string
	replacement string
	length      int
}

func findWildcardMatch(mapping map[string]any, input string) (key string, replacement string, ok bool) {

	var best match

	for k := range mapping {
		idx := strings.IndexRune(k, '*')
		if idx == -1 {
			continue
		}

		before := k[:idx]
		after := k[idx+1:]

		if strings.HasPrefix(input, before) && strings.HasSuffix(input, after) {
			wild := input[len(before) : len(input)-len(after)]
			length := len(before) + len(after)

			if length > best.length {
				best = match{
					key:         k,
					replacement: wild,
					length:      length,
				}
			}
		}
	}

	if best.key == "" {
		return "", "", false
	}
	return best.key, best.replacement, true
}

func resolveMappingValue(value any, conditions []string) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []any:
		var result []string
		for _, item := range v {
			sub := resolveMappingValue(item, conditions)
			if sub != nil {
				result = append(result, sub...)
			}
		}
		return result
	case map[string]any:
		for _, cond := range conditions {
			if sub, exists := v[cond]; exists {
				return resolveMappingValue(sub, conditions)
			}
		}
	}

	return nil
}

func resolveMapping(mapping map[string]any, conditions []string, input string) []string {
	if value, ok := mapping[input]; ok {
		return resolveMappingValue(value, conditions)
	}

	key, wildcard, ok := findWildcardMatch(mapping, input)
	if !ok {
		return nil
	}

	value := mapping[key]
	resolved := resolveMappingValue(value, conditions)
	if resolved == nil {
		return nil
	}

	result := make([]string, 0, len(resolved))
	for _, r := range resolved {
		if strings.ContainsRune(r, '*') {
			result = append(result, strings.Replace(r, "*", wildcard, 1))
		} else {
			result = append(result, r)
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
	Imports    any
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
		Imports:    NormalizeMapping(config.Imports),
	}
}

func (r *SubpathResolver) ResolveExports(entry string) []string {
	if r.Exports == nil {
		return nil
	}
	return resolveMapping(r.Exports, r.Conditions, normalizeEntry(entry))
}

func (r *SubpathResolver) ResolveImports(entry string) []string {
	if r.Imports == nil {
		return nil
	}
	return resolveMapping(r.Imports, r.Conditions, entry)
}
