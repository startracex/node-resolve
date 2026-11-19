package resolve

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type memFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (i memFileInfo) Name() string       { return i.name }
func (i memFileInfo) Size() int64        { return i.size }
func (i memFileInfo) Mode() fs.FileMode  { return 0 }
func (i memFileInfo) ModTime() time.Time { return time.Time{} }
func (i memFileInfo) IsDir() bool        { return i.isDir }
func (i memFileInfo) Sys() any           { return nil }

type memFS struct {
	files map[string]string
	dirs  map[string]struct{}
}

func newMemFS() *memFS {
	return &memFS{
		files: map[string]string{},
		dirs:  map[string]struct{}{},
	}
}

func (m *memFS) addFile(path, content string) {
	path = filepath.Clean(path)
	m.files[path] = content
	dir := filepath.Dir(path)
	for dir != "." && dir != string(filepath.Separator) {
		m.dirs[dir] = struct{}{}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

func (m *memFS) Stat(path string) (fs.FileInfo, error) {
	path = filepath.Clean(path)
	if content, ok := m.files[path]; ok {
		return memFileInfo{name: filepath.Base(path), size: int64(len(content)), isDir: false}, nil
	}
	if _, ok := m.dirs[path]; ok {
		return memFileInfo{name: filepath.Base(path), isDir: true}, nil
	}
	for file := range m.files {
		if strings.HasPrefix(file, path+string(filepath.Separator)) {
			return memFileInfo{name: filepath.Base(path), isDir: true}, nil
		}
	}
	return nil, fs.ErrNotExist
}

func (m *memFS) ReadFile(path string) ([]byte, error) {
	path = filepath.Clean(path)
	content, ok := m.files[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(content), nil
}

func TestResolveIndexFallback(t *testing.T) {
	fsys := newMemFS()
	fsys.addFile(filepath.Join("proj", "dir", "index.js"), "export {}")

	r := NewModuleResolver(ResolverConfig{
		Extensions: []string{".js"},
		IndexName:  "index",
		FS:         fsys,
	})

	got := r.Resolve("./dir", "proj")
	want := filepath.Join("proj", "dir", "index.js")
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveIndexDisabledWhenEmpty(t *testing.T) {
	fsys := newMemFS()
	fsys.addFile(filepath.Join("proj", "dir", "index.js"), "export {}")

	r := NewModuleResolver(ResolverConfig{
		Extensions: []string{".js"},
		IndexName:  "",
		FS:         fsys,
	})

	got := r.Resolve("./dir", "proj")
	if got != "" {
		t.Fatalf("Resolve() = %q, want empty", got)
	}
}

func TestResolveExtensionMapOrder(t *testing.T) {
	fsys := newMemFS()
	fsys.addFile(filepath.Join("proj", "mod-file.js"), "js")
	fsys.addFile(filepath.Join("proj", "mod-file.ts"), "ts")

	r := NewModuleResolver(ResolverConfig{
		Extensions: []string{".js", ".ts"},
		ExtensionMap: map[string][]string{
			".js": {".ts", ".js"},
		},
		FS: fsys,
	})

	got := r.Resolve("./mod-file.js", "proj")
	want := filepath.Join("proj", "mod-file.ts")
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveMainAndIndexFallback(t *testing.T) {
	fsys := newMemFS()
	fsys.addFile(filepath.Join("proj", "pkg", "package.json"), `{"main":"./missing.js"}`)
	fsys.addFile(filepath.Join("proj", "pkg", "index.js"), "export {}")

	r := NewModuleResolver(ResolverConfig{
		Extensions: []string{".js"},
		IndexName:  "index",
		MainFields: []string{"main"},
		FS:         fsys,
	})

	got := r.Resolve("./pkg", "proj")
	want := filepath.Join("proj", "pkg", "index.js")
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveMainDirectory(t *testing.T) {
	fsys := newMemFS()
	fsys.addFile(filepath.Join("proj", "pkg", "package.json"), `{"main":"./lib"}`)
	fsys.addFile(filepath.Join("proj", "pkg", "lib", "index.js"), "export {}")

	r := NewModuleResolver(ResolverConfig{
		Extensions: []string{".js"},
		IndexName:  "index",
		MainFields: []string{"main"},
		FS:         fsys,
	})

	got := r.Resolve("./pkg", "proj")
	want := filepath.Join("proj", "pkg", "lib", "index.js")
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveExports(t *testing.T) {
	fsys := newMemFS()
	fsys.addFile(filepath.Join("proj", "node_modules", "demo", "package.json"), `{
		"exports": {
			".": "./dist/main.js",
			"./util": "./dist/util.js"
		}
	}`)
	fsys.addFile(filepath.Join("proj", "node_modules", "demo", "dist", "main.js"), "")
	fsys.addFile(filepath.Join("proj", "node_modules", "demo", "dist", "util.js"), "")

	r := NewModuleResolver(ResolverConfig{
		Extensions: []string{".js"},
		FS:         fsys,
	})

	got := r.Resolve("demo", "proj")
	want := filepath.Join("proj", "node_modules", "demo", "dist", "main.js")
	if got != want {
		t.Fatalf("Resolve(demo) = %q, want %q", got, want)
	}

	got = r.Resolve("demo/util", "proj")
	want = filepath.Join("proj", "node_modules", "demo", "dist", "util.js")
	if got != want {
		t.Fatalf("Resolve(demo/util) = %q, want %q", got, want)
	}
}

func TestResolveTopLevelConditionalExports(t *testing.T) {
	fsys := newMemFS()
	fsys.addFile(filepath.Join("proj", "node_modules", "demo", "package.json"), `{
		"exports": {
			"import": "./esm.js",
			"require": "./cjs.js",
			"default": "./index.js"
		}
	}`)
	fsys.addFile(filepath.Join("proj", "node_modules", "demo", "esm.js"), "")
	fsys.addFile(filepath.Join("proj", "node_modules", "demo", "cjs.js"), "")
	fsys.addFile(filepath.Join("proj", "node_modules", "demo", "index.js"), "")

	r := NewModuleResolver(ResolverConfig{
		Extensions: []string{".js"},
		Conditions: []string{"import"},
		FS:         fsys,
	})

	got := r.Resolve("demo", "proj")
	want := filepath.Join("proj", "node_modules", "demo", "esm.js")
	if got != want {
		t.Fatalf("Resolve(demo) = %q, want %q", got, want)
	}
}

func TestResolveImportsFromPackageRoot(t *testing.T) {
	fsys := newMemFS()
	fsys.addFile(filepath.Join("proj", "package.json"), `{
		"imports": {
			"#lib/*": "./src/*.js"
		}
	}`)
	fsys.addFile(filepath.Join("proj", "src", "math.js"), "")

	r := NewModuleResolver(ResolverConfig{
		Extensions: []string{".js"},
		FS:         fsys,
	})

	got := r.Resolve("#lib/math", filepath.Join("proj", "nested"))
	want := filepath.Join("proj", "src", "math.js")
	if got != want {
		t.Fatalf("Resolve(#lib/math) = %q, want %q", got, want)
	}
}

func TestResolveCoreModuleUsesRequest(t *testing.T) {
	r := NewModuleResolver(ResolverConfig{})
	got := r.Resolve("node:fs", "proj")
	if got != "node:fs" {
		t.Fatalf("Resolve(node:fs) = %q, want node:fs", got)
	}
}

func TestNormalizeMappingAnySlice(t *testing.T) {
	got := NormalizeMapping([]any{"./a.js", "./b.js"})
	want := map[string]any{".": []any{"./a.js", "./b.js"}}
	if len(got) != 1 {
		t.Fatalf("NormalizeMapping() = %#v", got)
	}
	arr, ok := got["."].([]any)
	if !ok || len(arr) != 2 || arr[0] != "./a.js" || arr[1] != "./b.js" {
		t.Fatalf("NormalizeMapping() = %#v, want %#v", got, want)
	}
}
