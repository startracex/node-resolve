# node-resolve

```ts
import { ModuleResolver } from "@startracex/node-resolve";
import fs from "node:fs";
import path from "node:path";

const resolver = new ModuleResolver({
  extensions: [".js"],
  isCoreModule: (id) => id.startsWith("node:"),
  mainFields: ["main"],
  path: path,
  fs: fs,
});

const resolved = resolver.resolve("@scope/pkg", "/dir/of/resolver"); // /dir/node_modules/@scope/pkg/index.js
```

```go
import (
	"strings"
	resolve "github.com/startracex/node-resolve"
)

func main() {
	resolver := resolve.NewModuleResolver(&resolve.ResolverConfig{
		Extensions: []string{".js"},
		IsCoreModule: func(s string) bool {
			return strings.HasPrefix(s, "node:")
		},
		MainFields: []string{"main"},
	})
	resolved := resolver.Resolve("@scope/pkg", "/dir/of/resolver") // /dir/node_modules/@scope/pkg/index.js
}
```
