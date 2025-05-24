package resolve

import (
	"fmt"
	"regexp"
	"strings"
)

type Specifier struct {
	Proto string
	Scope string
	Pkg   string
	Name  string
	Path  string
}

var (
	SpecifierRegex = regexp.MustCompile(`^(?:@(\w[\w-.]*)/)?(\w[\w-.]*)(/(.*))?$`)

	ErrInvalidSpecifier = fmt.Errorf("resolve: invalid specifier")
)

func ParseSpecifier(input string) (*Specifier, error) {
	if input == "" {
		return nil, ErrInvalidSpecifier
	}

	sp := strings.SplitN(input, ":", 2)

	var proto, name string

	if len(sp) > 1 {
		proto = sp[0]
		input = strings.Join(sp[1:], ":")
	}

	match := SpecifierRegex.FindStringSubmatch(input)
	if match == nil || match[0] == "" || match[2] == "" {
		return nil, ErrInvalidSpecifier
	}

	scope := match[1]
	pkg := match[2]
	path := match[4]

	if scope != "" {
		name = fmt.Sprintf("@%s/%s", scope, pkg)
	} else {
		name = pkg
	}

	return &Specifier{
		Proto: proto,
		Scope: scope,
		Pkg:   pkg,
		Path:  path,
		Name:  name,
	}, nil
}
