package resolve

import (
	"reflect"
	"testing"
)

func TestNormalizeMapping(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  map[string]any
	}{
		{
			name:  "nil input",
			input: nil,
			want:  map[string]any{},
		},
		{
			name:  "string input",
			input: "test",
			want:  map[string]any{".": "test"},
		},
		{
			name:  "string slice input",
			input: []string{"a", "b"},
			want:  map[string]any{".": []string{"a", "b"}},
		},
		{
			name: "map input",
			input: map[string]any{
				".": "value",
			},
			want: map[string]any{
				".": "value",
			},
		},
		{
			name:  "unsupported type",
			input: 0,
			want:  map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMapping(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeMapping(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindWildcardMatch(t *testing.T) {
	mapping := map[string]any{
		"a*":            1,
		"b*c":           2,
		"b*d":           3,
		"noStar":        4,
		"prefix*suffix": 5,
	}

	tests := []struct {
		input     string
		wantKey   string
		wantWild  string
		wantFound bool
	}{
		{"abc", "a*", "bc", true},
		{"bxc", "b*c", "x", true},
		{"prefixXYZsuffix", "prefix*suffix", "XYZ", true},
		{"noMatchHere", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			key, wild, ok := findWildcardMatch(mapping, tt.input)
			if ok != tt.wantFound || key != tt.wantKey || wild != tt.wantWild {
				t.Errorf("findWildcardMatch(%v) = (%v, %v, %v), want (%v, %v, %v)",
					tt.input, key, wild, ok, tt.wantKey, tt.wantWild, tt.wantFound)
			}
		})
	}
}

func TestResolveMappingValue(t *testing.T) {
	tests := []struct {
		name       string
		value      any
		conditions []string
		want       []string
	}{
		{
			name:       "string value",
			value:      "foo",
			conditions: []string{"default"},
			want:       []string{"foo"},
		},
		{
			name:       "array of any",
			value:      []any{"a", "b"},
			conditions: []string{"default"},
			want:       []string{"a", "b"},
		},
		{
			name: "nested map match condition",
			value: map[string]any{
				"node": "nodeValue",
				"def":  "defaultValue",
			},
			conditions: []string{"def"},
			want:       []string{"defaultValue"},
		},
		{
			name:       "unsupported type",
			value:      123,
			conditions: []string{"default"},
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveMappingValue(tt.value, tt.conditions)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveMappingValue(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestResolveMapping(t *testing.T) {
	mapping := map[string]any{
		"foo": "bar",
		"pkg/*": []any{
			"dist/*.js",
			"alt/*.mjs",
		},
	}

	tests := []struct {
		name       string
		input      string
		conditions []string
		want       []string
	}{
		{
			name:       "direct match",
			input:      "foo",
			conditions: []string{"default"},
			want:       []string{"bar"},
		},
		{
			name:       "wildcard match",
			input:      "pkg/util",
			conditions: []string{"default"},
			want:       []string{"dist/util.js", "alt/util.mjs"},
		},
		{
			name:       "no match",
			input:      "unknown",
			conditions: []string{"default"},
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveMapping(mapping, tt.conditions, tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveMapping(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeEntry(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "."},
		{".", "."},
		{"./", "."},
		{"./test", "./test"},
		{"test", "./test"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeEntry(tt.input)
			if got != tt.want {
				t.Errorf("normalizeEntry(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewSubpathResolver(t *testing.T) {
	t.Run("default conditions", func(t *testing.T) {
		r := NewSubpathResolver(SubpathResolverConfig{})
		if !reflect.DeepEqual(r.Conditions, []string{"default"}) {
			t.Errorf("expected default condition, got %v", r.Conditions)
		}
	})

	t.Run("custom conditions and mappings", func(t *testing.T) {
		cfg := SubpathResolverConfig{
			Exports:    "exportVal",
			Imports:    "importVal",
			Conditions: []string{"node"},
		}
		r := NewSubpathResolver(cfg)
		if !reflect.DeepEqual(r.Conditions, []string{"node"}) {
			t.Errorf("expected node condition, got %v", r.Conditions)
		}
		if r.Exports["."] != "exportVal" || r.Imports["."] != "importVal" {
			t.Errorf("expected normalized maps, got %+v %+v", r.Exports, r.Imports)
		}
	})
}

func TestResolveExportsAndImports(t *testing.T) {
	r := &SubpathResolver{
		Conditions: []string{"default"},
		Exports: map[string]any{
			".":      "main.js",
			"./util": []any{"util.js"},
		},
		Imports: map[string]any{
			"#pkg/*": "lib/*.js",
		},
	}

	t.Run("resolve exports", func(t *testing.T) {
		got := r.ResolveExports("util")
		want := []string{"util.js"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ResolveExports() = %v, want %v", got, want)
		}
	})

	t.Run("resolve imports", func(t *testing.T) {
		got := r.ResolveImports("#pkg/math")
		want := []string{"lib/math.js"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ResolveImports() = %v, want %v", got, want)
		}
	})
}
