# node-resolve

```go
import (
	"strings"
	"os"

	resolve "github.com/startracex/node-resolve"
)

func main() {
	resolver := resolve.NewModuleResolver(&resolve.ResolverConfig{
		Extensions: []string{".js", ".ts"},
		ExtensionMap: map[string][]string{
			".js": {".ts", ".js"},
		},
	})
	cwd, _ := os.Getwd()
	resolved := ""
	resolved = resolver.Resolve("./mod-file.js", cwd) // mod-file.ts
	resolved = resolver.Resolve("typescript", cwd) // node_modules/typescript/lib/typescript.js
}
```
