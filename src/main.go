package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall/js"
	"time"

	resolve "github.com/startracex/node-resolve"
)

type jsFileInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
}

func (v *jsFileInfo) Name() string       { return v.name }
func (v *jsFileInfo) Size() int64        { return v.size }
func (v *jsFileInfo) Mode() fs.FileMode  { return 0 }
func (v *jsFileInfo) ModTime() time.Time { return v.modTime }
func (v *jsFileInfo) IsDir() bool        { return v.isDir }
func (v *jsFileInfo) Sys() any           { return nil }

type jsFS struct {
	jsObj js.Value
}

func (f jsFS) Stat(path string) (fs.FileInfo, error) {
	result := f.jsObj.Call("stat", path)
	if !result.Get("exists").Bool() {
		return nil, os.ErrNotExist
	}
	return &jsFileInfo{
		name:    filepath.Base(path),
		size:    int64(result.Get("size").Int()),
		isDir:   result.Get("isDir").Bool(),
		modTime: time.UnixMilli(int64(result.Get("mtime").Int())),
	}, nil
}

func (f jsFS) ReadFile(path string) ([]byte, error) {
	return []byte(f.jsObj.Call("readFile", path).String()), nil
}

type jsPath struct {
	jsObj js.Value
}

func (p jsPath) Dir(path string) string {
	return p.jsObj.Call("dir", path).String()
}

func (p jsPath) Join(elem ...string) string {
	jsArgs := make([]any, len(elem))
	for i, e := range elem {
		jsArgs[i] = e
	}
	return p.jsObj.Call("join", jsArgs...).String()
}

func toStringSlice(jsVal js.Value) []string {
	if jsVal.IsUndefined() || jsVal.IsNull() {
		return nil
	}
	if jsVal.Type() != js.TypeObject || !jsVal.InstanceOf(js.Global().Get("Array")) {
		return nil
	}
	length := jsVal.Length()
	result := make([]string, length)
	for i := 0; i < length; i++ {
		result[i] = jsVal.Index(i).String()
	}
	return result
}

func toStringSliceMap(jsObj js.Value) map[string][]string {
	if jsObj.IsUndefined() || jsObj.IsNull() {
		return nil
	}
	if jsObj.Type() != js.TypeObject {
		return nil
	}
	result := make(map[string][]string)
	keys := js.Global().Get("Object").Call("keys", jsObj)
	for i := 0; i < keys.Length(); i++ {
		key := keys.Index(i).String()
		val := jsObj.Get(key)
		if val.Type() == js.TypeObject && val.InstanceOf(js.Global().Get("Array")) {
			result[key] = toStringSlice(val)
		}
	}
	return result
}

func JSResolve(this js.Value, args []js.Value) any {
	arg0 := args[0]
	fs := jsFS{jsObj: arg0.Get("fs")}
	path := jsPath{jsObj: arg0.Get("path")}
	resolver := resolve.NewModuleResolver(&resolve.ResolverConfig{
		Extensions:           toStringSlice(arg0.Get("extensions")),
		ExtensionMap:         toStringSliceMap(arg0.Get("extensionMap")),
		ModulesDirectoryName: arg0.Get("modulesDirectoryName").String(),
		ManifestFileName:     arg0.Get("manifestFileName").String(),
		MainFields:           toStringSlice(arg0.Get("mainFields")),
		IndexName:            arg0.Get("indexName").String(),
		Conditions:           toStringSlice(arg0.Get("conditions")),
		FS:                   fs,
		Path:                 path,
		IsCoreModule: func(s string) bool {
			return arg0.Call("isCoreModule", s).Bool()
		},
	})

	return js.FuncOf(func(this js.Value, args []js.Value) any {
		return resolver.Resolve(args[0].String(), args[1].String())
	})
}

func main() {
	js.Global().Set("@startracex/node-resolve", js.FuncOf(JSResolve))
	select {}
}
