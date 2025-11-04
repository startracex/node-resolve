package resolve

import (
	"reflect"
	"testing"
)

func TestParseSpecifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Specifier
		wantErr error
	}{
		{
			name:    "empty input",
			input:   "",
			want:    nil,
			wantErr: ErrInvalidSpecifier,
		},
		{
			name:    "invalid format",
			input:   "???",
			want:    nil,
			wantErr: ErrInvalidSpecifier,
		},
		{
			name:  "simple package",
			input: "mypkg",
			want: &Specifier{
				Proto: "",
				Scope: "",
				Pkg:   "mypkg",
				Path:  "",
				Name:  "mypkg",
			},
		},
		{
			name:  "proto and package",
			input: "npm:mypkg",
			want: &Specifier{
				Proto: "npm",
				Scope: "",
				Pkg:   "mypkg",
				Path:  "",
				Name:  "mypkg",
			},
		},
		{
			name:  "scoped package",
			input: "@scope/pkgname",
			want: &Specifier{
				Proto: "",
				Scope: "@scope",
				Pkg:   "pkgname",
				Path:  "",
				Name:  "@scope/pkgname",
			},
		},
		{
			name:  "proto and scoped package",
			input: "npm:@scope/pkgname",
			want: &Specifier{
				Proto: "npm",
				Scope: "@scope",
				Pkg:   "pkgname",
				Path:  "",
				Name:  "@scope/pkgname",
			},
		},
		{
			name:  "package with path",
			input: "pkgname/sub/path",
			want: &Specifier{
				Proto: "",
				Scope: "",
				Pkg:   "pkgname",
				Path:  "sub/path",
				Name:  "pkgname",
			},
		},
		{
			name:  "scoped package with path",
			input: "@scope/pkgname/sub/path",
			want: &Specifier{
				Proto: "",
				Scope: "@scope",
				Pkg:   "pkgname",
				Path:  "sub/path",
				Name:  "@scope/pkgname",
			},
		},
		{
			name:  "proto, scope, package, and path",
			input: "git:@org/repo/src/utils",
			want: &Specifier{
				Proto: "git",
				Scope: "@org",
				Pkg:   "repo",
				Path:  "src/utils",
				Name:  "@org/repo",
			},
		},
		{
			name:    "missing package name after slash",
			input:   "@scope/",
			want:    nil,
			wantErr: ErrInvalidSpecifier,
		},
		{
			name:    "proto with missing package",
			input:   "npm:",
			want:    nil,
			wantErr: ErrInvalidSpecifier,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSpecifier(tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if err != tt.wantErr {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
