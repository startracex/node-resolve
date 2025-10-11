# node-resolve

```js
import { createResolve } from "./npm/index.mjs";

const resolve = await createResolve({
  extensions: [".js", ".ts"],
  extensionMap: {
    ".js": [".ts", ".js"],
  },
});

let resolved, cwd = process.cwd()
resolved = resolve("./file.ts", cwd); // file.js
resolved = resolve("typescript", cwd); // node_modules/typescript/lib/typescript.js
```
