# node-resolve

```js
import { createResolve } from "@startracex/node-resolve";

const resolve = await createResolve({
  extensions: [".js", ".ts"],
  extensionMap: {
    ".js": [".ts", ".js"],
  },
});

let resolved, cwd = process.cwd()
resolved = resolve("./file.js", cwd); // file.ts
resolved = resolve("typescript", cwd); // node_modules/typescript/lib/typescript.js
```
