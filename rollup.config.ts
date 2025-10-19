import { globSync } from "node:fs";
import type { RollupOptions } from "rollup";
import cjsShim from "rollup-plugin-cjs-shim";
import oxc from "rollup-plugin-oxc";

const input = globSync("src/**/*.[tj]s");
const external = /^node:/;

export default [
  {
    input,
    output: {
      dir: "npm",
      format: "esm",
      sourcemap: true,
      hoistTransitiveImports: false,
    },
    external,
    plugins: [
      oxc({
        minify: true,
      }),
    ],
  },
  {
    input,
    output: {
      dir: "npm",
      format: "cjs",
      sourcemap: true,
      entryFileNames: "[name].cjs",
      hoistTransitiveImports: false,
    },
    external,
    plugins: [
      cjsShim(),
      oxc({
        minify: true,
        declaration: false,
      }),
    ],
  },
] as RollupOptions[];
