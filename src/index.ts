import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { readFileSync } from "node:fs";
import { type Options, normalizeOptions } from "./shared.js";
import { goGlobal, Go } from "./wasm_exec.js";

const go = new Go();
let loaded = false;
const init = async () => {
  if (loaded) {
    return;
  }
  const wasmPath = join(dirname(fileURLToPath(import.meta.url)), "main.wasm");
  const buffer = readFileSync(wasmPath);
  const result = await WebAssembly.instantiate(buffer, go.importObject);
  go.run(result.instance);
  loaded = true;
};

export const createResolve = async (
  options?: Options,
): Promise<(req: string, dir: string) => string> => {
  await init();
  return goGlobal["@startracex/node-resolve"](normalizeOptions(options));
};
