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
	candidates := map[string]struct{}{
		filePath: {},
	}
	filePathExt := path.Ext(filePath)
	if exts, ok := r.ExtensionMap[filePathExt]; ok {
		base := filePath[:len(filePath)-len(filePathExt)]
		for _, ext := range exts {
			candidates[base+ext] = struct{}{}
		}
	}

	for _, ext := range r.Extensions {
		candidates[filePath+ext] = struct{}{}
	}
	for file := range candidates {
		stat, err := r.FS.Stat(file)
		if err != nil || stat.IsDir() {
			continue
		}
		if !stat.IsDir() {
			return file
		}
	}

	return ""
}

func (r *ModuleResolver) resolveDir(dirPath string, entry string) string {
	packageJSONPath := r.Path.Join(dirPath, r.ManifestFileName)
	stat, err := r.FS.Stat(packageJSONPath)

	if err != nil || stat.IsDir() {
		if r.IndexName != "" {
			return ""
		}
		return r.resolveFile(r.Path.Join(dirPath, r.IndexName))
	}

	var pkg map[string]any
	if err = readJSON(r.FS, packageJSONPath, &pkg); err != nil {
		if r.IndexName != "" {
			return ""
		}
		return r.resolveFile(r.Path.Join(dirPath, r.IndexName))
	}

	if exports, ok := pkg["exports"]; ok {
		exportsResolver := NewSubpathResolver(SubpathResolverConfig{
			Exports:    exports,
			Conditions: r.Conditions,
		})
		subpathResolved := exportsResolver.ResolveExports(entry)
		return r.resolveDirSubpaths(dirPath, subpathResolved)
	}

	if entry == "" {
		for _, field := range r.MainFields {
			if main, ok := pkg[field].(string); ok && main != "" {
				mainPath := r.Path.Join(dirPath, main)
				stat, err := r.FS.Stat(mainPath)
				if err == nil && !stat.IsDir() {
					return mainPath
				}
			}
		}
		return ""
	}

	subPath := r.Path.Join(dirPath, entry)
	return r.resolveFileOrDir(subPath, entry)
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
	path, err := r.FindUp(dir, r.ManifestFileName)
	if err != nil {
		return nil, err
	}
	var manifest map[string]any
	return manifest, readJSON(r.FS, path, &manifest)
}

func (r *ModuleResolver) Resolve(path string, dir string) string {
	if strings.HasPrefix(path, "#") {
		return r.ResolveImports(path, dir)
	}
	spec, err := NewSpecifier(path)
	if err == nil && spec.Name != "" {
		if r.IsCoreModule(spec.Name) {
			return path
		}

		return r.ResolveModuleSpecifier(spec, dir)
	}

	return r.resolveFileOrDir(r.Path.Join(dir, path), "")
}

func (r *ModuleResolver) ResolveImports(path, dir string) string {
	manifest, err := r.FindManifest(dir)
	if err != nil {
		return ""
	}
	if imports, ok := manifest["imports"]; ok {
		subpathResolver := NewSubpathResolver(SubpathResolverConfig{
			Imports:    imports,
			Conditions: r.Conditions,
		})
		subpathResolved := subpathResolver.ResolveImports(path)
		return r.resolveDirSubpaths(dir, subpathResolved)
	}
	return ""
}

func (r *ModuleResolver) ResolveModuleSpecifier(spec *Specifier, dir string) string {
	dirs := r.ModulesPaths(dir, spec.Name)
	for _, dir := range dirs {
		stat, err := r.FS.Stat(dir)
		if err == nil && stat.IsDir() {
			rd := r.resolveDir(dir, spec.Path)
			if rd != "" {
				return rd
			}
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
