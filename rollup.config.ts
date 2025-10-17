import type { RollupOptions } from "rollup";
import cjsShim from "rollup-plugin-cjs-shim";
import oxc from "rollup-plugin-oxc";

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
      cjsShim(),
      oxc({
        minify: true,
        declaration: false,
      }),
    ],
  },
] as RollupOptions[];
