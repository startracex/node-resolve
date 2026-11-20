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
	IsAbs(path string) bool
}

type osPath struct {
}

func (*osPath) Dir(path string) string {
	return filepath.Dir(path)
}

func (*osPath) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (*osPath) IsAbs(path string) bool {
	return filepath.IsAbs(path)
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

func (r *ModuleResolver) isModulesDirectorySegment(dir string) bool {
	parent := r.Path.Dir(dir)
	if parent == dir {
		return dir == r.ModulesDirectoryName
	}
	return r.Path.Join(parent, r.ModulesDirectoryName) == dir
}

// NodeModulesPaths returns node_modules directories to search, matching NODE_MODULES_PATHS.
// Segments already named ModulesDirectoryName are skipped.
func (r *ModuleResolver) NodeModulesPaths(start string) []string {
	var dirs []string
	if start == "" {
		return dirs
	}
	dir := start
	for {
		if !r.isModulesDirectorySegment(dir) {
			dirs = append(dirs, r.Path.Join(dir, r.ModulesDirectoryName))
		}
		parent := r.Path.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

func (r *ModuleResolver) ModulesPaths(start string, name string) []string {
	nms := r.NodeModulesPaths(start)
	paths := make([]string, len(nms))
	for i, nm := range nms {
		paths[i] = r.Path.Join(nm, name)
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

// resolveDir implements LOAD_AS_DIRECTORY (main / index only; not exports).
func (r *ModuleResolver) resolveDir(dirPath string) string {
	pkg, ok := r.loadPackage(r.Path.Join(dirPath, r.ManifestFileName))
	if !ok {
		return r.resolveIndex(dirPath)
	}

	for _, field := range r.MainFields {
		if main, ok := pkg[field].(string); ok && main != "" {
			mainPath := r.Path.Join(dirPath, main)
			if resolved := r.resolveFileOrDir(mainPath); resolved != "" {
				return resolved
			}
		}
	}
	return r.resolveIndex(dirPath)
}

// resolveExactFile implements RESOLVE_ESM_MATCH: the target must exist as a file.
func (r *ModuleResolver) resolveExactFile(filePath string) string {
	stat, err := r.FS.Stat(filePath)
	if err != nil || stat.IsDir() {
		return ""
	}
	return filePath
}

func (r *ModuleResolver) resolveDirSubpaths(dir string, targets []string) string {
	for _, match := range targets {
		if match == "" {
			continue
		}
		if resolved := r.resolveExactFile(r.Path.Join(dir, match)); resolved != "" {
			return resolved
		}
	}
	return ""
}

func (r *ModuleResolver) resolveFileOrDir(subPath string) string {
	if file := r.resolveFile(subPath); file != "" {
		return file
	}
	return r.resolveDir(subPath)
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
	if r.Path.IsAbs(request) {
		return r.resolveFileOrDir(request)
	}

	if IsRelPath(request) {
		return r.resolveFileOrDir(r.Path.Join(dir, request))
	}

	if strings.HasPrefix(request, "#") {
		return r.ResolveImports(request, dir)
	}

	if r.IsCoreModule(request) {
		return request
	}

	spec, err := NewSpecifier(request)
	if err != nil || spec.Name == "" {
		return r.resolveFileOrDir(r.Path.Join(dir, request))
	}

	// LOAD_PACKAGE_SELF: if handled (name+exports), do not fall through.
	if result, stop := r.ResolvePackageSelf(spec, dir); stop {
		return result
	}

	return r.ResolveModuleSpecifier(spec, dir)
}

// loadPackageExports implements LOAD_PACKAGE_EXPORTS.
// handled=false means exports is absent/null (fall through).
// handled=true means exports was consulted; result may be empty (like throw).
func (r *ModuleResolver) loadPackageExports(packageDir, entry string) (string, bool) {
	pkg, ok := r.loadPackage(r.Path.Join(packageDir, r.ManifestFileName))
	if !ok {
		return "", false
	}
	exports, hasExports := pkg["exports"]
	if !hasExports || exports == nil {
		return "", false
	}
	exportsResolver := NewSubpathResolver(SubpathResolverConfig{
		Exports:    exports,
		Conditions: r.Conditions,
	})
	return r.resolveDirSubpaths(packageDir, exportsResolver.ResolveExports(entry)), true
}

// ResolvePackageSelf implements LOAD_PACKAGE_SELF.
// stop=true means the self package matched and resolution must not continue.
func (r *ModuleResolver) ResolvePackageSelf(spec *Specifier, dir string) (string, bool) {
	manifestPath, err := r.FindUp(dir, r.ManifestFileName)
	if err != nil {
		return "", false
	}
	pkg, ok := r.loadPackage(manifestPath)
	if !ok {
		return "", false
	}
	exports, hasExports := pkg["exports"]
	if !hasExports || exports == nil {
		return "", false
	}
	name, _ := pkg["name"].(string)
	if name == "" || name != spec.Name {
		return "", false
	}
	pkgRoot := r.Path.Dir(manifestPath)
	exportsResolver := NewSubpathResolver(SubpathResolverConfig{
		Exports:    exports,
		Conditions: r.Conditions,
	})
	return r.resolveDirSubpaths(pkgRoot, exportsResolver.ResolveExports(spec.Path)), true
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
	if !hasImports || imports == nil {
		return ""
	}
	importsResolver := NewSubpathResolver(SubpathResolverConfig{
		Imports:    imports,
		Conditions: r.Conditions,
	})
	return r.resolveImportTargets(pkgRoot, importsResolver.ResolveImports(request))
}

func (r *ModuleResolver) resolveImportTargets(pkgRoot string, targets []string) string {
	for _, match := range targets {
		if match == "" {
			continue
		}
		if r.Path.IsAbs(match) {
			if resolved := r.resolveExactFile(match); resolved != "" {
				return resolved
			}
			continue
		}
		if IsRelPath(match) {
			if resolved := r.resolveExactFile(r.Path.Join(pkgRoot, match)); resolved != "" {
				return resolved
			}
			continue
		}
		// Bare / package targets are resolved again from the package scope.
		if resolved := r.Resolve(match, pkgRoot); resolved != "" {
			return resolved
		}
	}
	return ""
}

// ResolveModuleSpecifier implements LOAD_NODE_MODULES.
func (r *ModuleResolver) ResolveModuleSpecifier(spec *Specifier, dir string) string {
	if dir == "" {
		return ""
	}

	for _, modulesDir := range r.NodeModulesPaths(dir) {
		packageDir := r.Path.Join(modulesDir, spec.Name)

		if resolved, handled := r.loadPackageExports(packageDir, spec.Path); handled {
			return resolved
		}

		target := packageDir
		if spec.Path != "" {
			target = r.Path.Join(packageDir, spec.Path)
		}
		if resolved := r.resolveFileOrDir(target); resolved != "" {
			return resolved
		}
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

// IsRelPath reports whether s looks like a filesystem path specifier.
func IsRelPath(s string) bool {
	if s == "" {
		return false
	}

	dot := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '/' || c == '\\' {
			return i != 0 && dot <= 2
		}
		if c == '.' && dot < 2 {
			dot++
			continue
		}
		return false
	}
	return dot == 1 || dot == 2
}
