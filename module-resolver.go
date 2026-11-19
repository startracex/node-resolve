package resolve

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type ModuleResolver struct {
	ResolverConfig
}

type FS interface {
	Stat(path string) (fs.FileInfo, error)
	ReadFile(path string) ([]byte, error)
}

type Path interface {
	Dir(path string) string
	Join(elem ...string) string
}

type osPath struct {
}

func (*osPath) Dir(path string) string {
	return filepath.Dir(path)
}

func (*osPath) Join(elem ...string) string {
	return filepath.Join(elem...)
}

type osFS struct {
}

func (*osFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (*osFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

type ResolverConfig struct {
	Extensions           []string
	ExtensionMap         map[string][]string
	IsCoreModule         func(string) bool
	ModulesDirectoryName string
	ManifestFileName     string
	MainFields           []string
	IndexName            string
	Conditions           []string
	FS                   FS
	Path                 Path
}

func NewModuleResolver(config ResolverConfig) *ModuleResolver {
	if config.ModulesDirectoryName == "" {
		config.ModulesDirectoryName = "node_modules"
	}
	if config.ManifestFileName == "" {
		config.ManifestFileName = "package.json"
	}
	if len(config.MainFields) == 0 {
		config.MainFields = []string{"main"}
	}
	config.Conditions = NormalizeConditions(config.Conditions)
	if config.IsCoreModule == nil {
		config.IsCoreModule = func(s string) bool {
			return strings.HasPrefix(s, "node:")
		}
	}
	if config.FS == nil {
		config.FS = &osFS{}
	}
	if config.Path == nil {
		config.Path = &osPath{}
	}

	return &ModuleResolver{
		config,
	}
}

func (r *ModuleResolver) ModulesPaths(start string, name string) []string {
	var paths []string

	if start == "" {
		return paths
	}

	for {
		paths = append(paths, r.Path.Join(start, r.ModulesDirectoryName, name))
		parent := r.Path.Dir(start)
		if parent == start {
			break
		}
		start = parent
	}

	return paths
}

func (r *ModuleResolver) resolveFile(filePath string) string {
	var candidates []string
	seen := map[string]struct{}{}
	add := func(p string) {
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		candidates = append(candidates, p)
	}

	filePathExt := path.Ext(filePath)
	if exts, ok := r.ExtensionMap[filePathExt]; ok {
		base := filePath[:len(filePath)-len(filePathExt)]
		for _, ext := range exts {
			add(base + ext)
		}
	} else {
		add(filePath)
	}

	for _, ext := range r.Extensions {
		add(filePath + ext)
	}

	for _, file := range candidates {
		stat, err := r.FS.Stat(file)
		if err != nil || stat.IsDir() {
			continue
		}
		return file
	}

	return ""
}

func (r *ModuleResolver) resolveIndex(dirPath string) string {
	if r.IndexName == "" {
		return ""
	}
	return r.resolveFile(r.Path.Join(dirPath, r.IndexName))
}

func (r *ModuleResolver) loadPackage(manifestPath string) (map[string]any, bool) {
	stat, err := r.FS.Stat(manifestPath)
	if err != nil || stat.IsDir() {
		return nil, false
	}

	var pkg map[string]any
	if err = readJSON(r.FS, manifestPath, &pkg); err != nil {
		return nil, false
	}
	return pkg, true
}

func (r *ModuleResolver) resolveDir(dirPath string, entry string) string {
	pkg, ok := r.loadPackage(r.Path.Join(dirPath, r.ManifestFileName))
	if !ok {
		return r.resolveIndex(dirPath)
	}

	if exports, hasExports := pkg["exports"]; hasExports {
		exportsResolver := NewSubpathResolver(SubpathResolverConfig{
			Exports:    exports,
			Conditions: r.Conditions,
		})
		return r.resolveDirSubpaths(dirPath, exportsResolver.ResolveExports(entry))
	}

	if entry == "" {
		for _, field := range r.MainFields {
			if main, ok := pkg[field].(string); ok && main != "" {
				mainPath := r.Path.Join(dirPath, main)
				if resolved := r.resolveFileOrDir(mainPath, ""); resolved != "" {
					return resolved
				}
			}
		}
		return r.resolveIndex(dirPath)
	}

	subPath := r.Path.Join(dirPath, entry)
	return r.resolveFileOrDir(subPath, "")
}

func (r *ModuleResolver) resolveDirSubpaths(dir string, s []string) string {
	for _, match := range s {
		matchPath := r.Path.Join(dir, match)
		stat, err := r.FS.Stat(matchPath)
		if err != nil || stat.IsDir() {
			continue
		}
		return matchPath
	}
	return ""
}

func (r *ModuleResolver) resolveFileOrDir(subPath string, entry string) string {
	if file := r.resolveFile(subPath); file != "" {
		return file
	}
	return r.resolveDir(subPath, entry)
}

func (r *ModuleResolver) FindManifest(dir string) (map[string]any, error) {
	manifestPath, err := r.FindUp(dir, r.ManifestFileName)
	if err != nil {
		return nil, err
	}
	pkg, ok := r.loadPackage(manifestPath)
	if !ok {
		return nil, ErrNoUpwardsFound
	}
	return pkg, nil
}

func (r *ModuleResolver) Resolve(request string, dir string) string {
	if strings.HasPrefix(request, "#") {
		return r.ResolveImports(request, dir)
	}
	spec, err := NewSpecifier(request)
	if err == nil && spec.Name != "" {
		if r.IsCoreModule(request) {
			return request
		}

		return r.ResolveModuleSpecifier(spec, dir)
	}

	return r.resolveFileOrDir(r.Path.Join(dir, request), "")
}

func (r *ModuleResolver) ResolveImports(request, dir string) string {
	manifestPath, err := r.FindUp(dir, r.ManifestFileName)
	if err != nil {
		return ""
	}
	pkgRoot := r.Path.Dir(manifestPath)
	pkg, ok := r.loadPackage(manifestPath)
	if !ok {
		return ""
	}
	imports, hasImports := pkg["imports"]
	if !hasImports {
		return ""
	}
	importsResolver := NewSubpathResolver(SubpathResolverConfig{
		Imports:    imports,
		Conditions: r.Conditions,
	})
	return r.resolveDirSubpaths(pkgRoot, importsResolver.ResolveImports(request))
}

func (r *ModuleResolver) ResolveModuleSpecifier(spec *Specifier, dir string) string {
	if dir == "" {
		return ""
	}

	// Walk parents lazily instead of allocating the full candidate list.
	start := dir
	for {
		candidate := r.Path.Join(start, r.ModulesDirectoryName, spec.Name)
		stat, err := r.FS.Stat(candidate)
		if err == nil && stat.IsDir() {
			if rd := r.resolveDir(candidate, spec.Path); rd != "" {
				return rd
			}
		}
		parent := r.Path.Dir(start)
		if parent == start {
			break
		}
		start = parent
	}
	return ""
}

var ErrNoUpwardsFound = errors.New("err no upwards found")

func (r *ModuleResolver) FindUp(startDir, target string) (string, error) {
	dir := startDir
	filepath := r.Path
	for {
		candidate := filepath.Join(dir, target)
		if _, err := r.FS.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ErrNoUpwardsFound
}

func readJSON(fs FS, path string, v any) error {
	data, err := fs.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}
	return nil
}
