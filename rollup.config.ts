import type { RollupOptions } from "rollup";
import oxc from "rollup-plugin-oxc";

function formatTransform() {
  return {
    name: "format-transform",
    transform(code: string) {
      code = code.replace("dirname(fileURLToPath(import.meta.url))", "__dirname");
      return { code, map: null };
    },
  };
}

export default [
  {
    input: {
      index: "src/index.ts",
      shared: "src/shared.ts",
      wasm_exec: "src/wasm_exec.js",
    },
    output: {
      dir: "npm",
      format: "esm",
      sourcemap: true,
      entryFileNames: "[name].mjs",
    },
    external: /^node:/,
    plugins: [
      oxc({
        minify: true,
      }),
    ],
  },
  {
    input: {
      index: "src/index.ts",
      shared: "src/shared.ts",
      wasm_exec: "src/wasm_exec.js",
    },
    output: {
      dir: "npm",
      format: "cjs",
      sourcemap: true,
      entryFileNames: "[name].cjs",
    },
    external: /^node:/,
    plugins: [
      formatTransform(),
      oxc({
        minify: true,
        declaration: false,
      }),
    ],
  },
] as RollupOptions[];
