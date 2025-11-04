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
	Config *ResolverConfig
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

func NewModuleResolver(config *ResolverConfig) *ModuleResolver {
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
		Config: config,
	}
}

func (r *ModuleResolver) ModulesPaths(start string, name string) []string {
	var paths []string

	if start == "" {
		return paths
	}

	for {
		paths = append(paths, r.Config.Path.Join(start, r.Config.ModulesDirectoryName, name))
		parent := r.Config.Path.Dir(start)
		if parent == start {
			break
		}
		start = parent
	}

	return paths
}

func (r *ModuleResolver) stat(path string) (fs.FileInfo, error) {
	return r.Config.FS.Stat(path)
}

func (r *ModuleResolver) resolveFile(filePath string) string {
	candidates := map[string]struct{}{
		filePath: {},
	}
	filePathExt := path.Ext(filePath)
	if exts, ok := r.Config.ExtensionMap[filePathExt]; ok {
		base := filePath[:len(filePath)-len(filePathExt)]
		for _, ext := range exts {
			candidates[base+ext] = struct{}{}
		}
	}

	for _, ext := range r.Config.Extensions {
		candidates[filePath+ext] = struct{}{}
	}
	for file := range candidates {
		stat, err := r.stat(file)
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
	packageJSONPath := r.Config.Path.Join(dirPath, r.Config.ManifestFileName)
	stat, err := r.stat(packageJSONPath)
	if err != nil || stat.IsDir() {
		return r.resolveFile(r.Config.Path.Join(dirPath, r.Config.IndexName))
	}

	pkg, err := r.readJSON(packageJSONPath)
	if err != nil {
		return r.resolveFile(r.Config.Path.Join(dirPath, r.Config.IndexName))
	}

	if exports, ok := pkg["exports"]; ok {
		exportsResolver := NewSubpathResolver(SubpathResolverConfig{
			Exports:    exports,
			Conditions: r.Config.Conditions,
		})
		exportsMatchArray := exportsResolver.ResolveExports(entry)

		for _, match := range exportsMatchArray {
			matchPath := r.Config.Path.Join(dirPath, match)
			stat, err := r.stat(matchPath)
			if err == nil && !stat.IsDir() {
				return matchPath
			}
		}

		return ""
	}

	if entry == "" {
		for _, field := range r.Config.MainFields {
			if main, ok := pkg[field].(string); ok && main != "" {
				mainPath := r.Config.Path.Join(dirPath, main)
				stat, err := r.stat(mainPath)
				if err == nil && !stat.IsDir() {
					return mainPath
				}
			}
		}
		return ""
	}

	subPath := r.Config.Path.Join(dirPath, entry)
	return r.resolveFileOrDir(subPath, entry)
}

func (r *ModuleResolver) resolveFileOrDir(subPath string, entry string) string {
	if file := r.resolveFile(subPath); file != "" {
		return file
	}
	return r.resolveDir(subPath, entry)
}

func (r *ModuleResolver) readJSON(path string) (map[string]any, error) {
	data, err := r.Config.FS.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *ModuleResolver) FindManifest(base string) (map[string]any, any) {
	p, err := r.FindUp(base, r.Config.ManifestFileName)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var manifest map[string]any
	err = json.Unmarshal(content, &manifest)
	if err != nil {
		return nil, err
	}
	return manifest, err
}

func (r *ModuleResolver) Resolve(path string, base string) string {
	if strings.HasPrefix(path, "#") {
		return r.ResolveImports(path, base)
	}
	spec, err := NewSpecifier(path)
	if err == nil && spec.Name != "" {
		if r.Config.IsCoreModule(spec.Name) {
			return path
		}

		return r.ResolveModuleSpecifier(spec, base)
	}

	return r.resolveFileOrDir(r.Config.Path.Join(base, path), "")
}

func (r *ModuleResolver) ResolveImports(path, base string) string {
	manifest, err := r.FindManifest(base)
	if err != nil {
		return ""
	}
	if imports, ok := manifest["imports"]; ok {
		subpathResolver := NewSubpathResolver(SubpathResolverConfig{
			Imports:    imports,
			Conditions: r.Config.Conditions,
		})
		subpathResolved := subpathResolver.ResolveImports(path)
		for _, file := range subpathResolved {
			file = r.Config.Path.Join(base, file)
			stat, err := r.stat(file)
			if err != nil || stat.IsDir() {
				continue
			}
			if !stat.IsDir() {
				return file
			}
		}
	}
	return ""
}

func (r *ModuleResolver) ResolveModuleSpecifier(spec *Specifier, base string) string {
	dirs := r.ModulesPaths(base, spec.Name)
	for _, dir := range dirs {
		stat, err := r.Config.FS.Stat(dir)
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
	filepath := r.Config.Path
	for {
		candidate := filepath.Join(dir, target)
		if _, err := r.stat(candidate); err == nil {
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
