package resolve

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

type ModuleResolver struct {
	*SubpathResolver
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
	IsCoreModule         func(string) bool
	ModulesDirectoryName string
	ManifestFileName     string
	MainFields           []string
	Base                 string
	FS                   FS
	Path                 Path
	SubpathResolverConfig
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
	if config.FS == nil {
		config.FS = &osFS{}
	}
	if config.Path == nil {
		config.Path = &osPath{}
	}

	return &ModuleResolver{
		SubpathResolver: NewSubpathResolver(config.SubpathResolverConfig),
		Config:          config,
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
	candidates := []string{filePath}
	if r.Config.Extensions != nil {
		for _, ext := range r.Config.Extensions {
			candidates = append(candidates, filePath+ext)
		}
	}

	for _, file := range candidates {
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
		return r.resolveFile(r.Config.Path.Join(dirPath, "index"))
	}

	pkg, err := r.readJSON(packageJSONPath)
	if err != nil {
		return r.resolveFile(r.Config.Path.Join(dirPath, "index"))
	}

	if exports, ok := pkg["exports"]; ok {
		exportsResolver := NewSubpathResolver(SubpathResolverConfig{
			Exports:    exports,
			Conditions: r.Conditions,
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

func (r *ModuleResolver) readJSON(path string) (map[string]interface{}, error) {
	data, err := r.Config.FS.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *ModuleResolver) Resolve(path string, base string) string {
	if  r.Config.IsCoreModule != nil && r.Config.IsCoreModule(path) {
		return path
	}

	spec, err := ParseSpecifier(path)
	if err == nil && spec.Name != "" {
		return r.ResolveModuleSpecifier(*spec, base)
	}

	return r.resolveFileOrDir(r.Config.Path.Join(base, path), "")
}

func (r *ModuleResolver) ResolveModuleSpecifier(spec Specifier, base string) string {
	dirs := r.ModulesPaths(base, spec.Name)
	for _, dir := range dirs {
		stat, err := r.stat(dir)
		if err == nil && stat.IsDir() {
			rd := r.resolveDir(dir, spec.Path)
			if rd != "" {
				return rd
			}
		}
	}
	return ""
}
