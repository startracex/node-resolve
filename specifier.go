package resolve

import (
	"errors"
	"path"
	"regexp"
)

type Specifier struct {
	Proto string
	Scope string
	Pkg   string
	Name  string
	Path  string
}

var (
	SpecifierRegex      = regexp.MustCompile(`^(?:([\w][\w0-9]*):)?(?:(@[a-z0-9-~][a-z0-9-._~]*)/)?([a-z0-9-][a-z0-9-._]*)(/(.*))?$`)
	ErrInvalidSpecifier = errors.New("resolve: invalid specifier")
)

func NewSpecifier(input string) (*Specifier, error) {
	if input == "" {
		return nil, ErrInvalidSpecifier
	}

	match := SpecifierRegex.FindStringSubmatch(input)
	if match == nil || match[0] == "" {
		return nil, ErrInvalidSpecifier
	}

	proto := match[1]
	scope := match[2]
	pkg := match[3]
	cleanedPath := match[5]
	if cleanedPath != "" {
		cleanedPath = path.Clean(cleanedPath)
	}

	if pkg == "" {
		return nil, ErrInvalidSpecifier
	}

	var name string
	if scope != "" {
		name = scope + "/" + pkg
	} else {
		name = pkg
	}

	return &Specifier{
		Proto: proto,
		Scope: scope,
		Pkg:   pkg,
		Path:  cleanedPath,
		Name:  name,
	}, nil
}
