import { readFileSync, statSync } from "node:fs";
import { dirname as dir, join } from "node:path";
import { builtinModules } from "node:module";

// replace by rollup-plugin-cjs-shim
const isESM = typeof import.meta !== "undefined";

const _isCoreModule = (id: string) =>
  id.startsWith("node:") || builtinModules.some((name) => id == name || id.startsWith(name + "/"));

const _fs = {
  stat: (path) => {
    try {
      const info = statSync(path);
      return {
        exists: true,
        isDir: info.isDirectory(),
        size: info.size,
        mtime: info.mtimeMs,
      };
    } catch {
      return {
        exists: false,
        isDir: false,
        size: 0,
        mtime: 0,
      };
    }
  },
  readFile: (path) => {
    return readFileSync(path, "utf-8");
  },
};

const _path = {
  dir,
  join,
};

export type Options = {
  extensions?: string[];
  extensionMap?: Record<string, string[]>;
  mainFields?: string[];
  conditions?: string[];
  indexName?: string;
  modulesDirectoryName?: string;
  manifestFileName?: string;
  isCoreModule?: (id: any) => boolean;
  path?: {
    dir: (path: string) => string;
    join: (...paths: string[]) => string;
  };
  fs?: {
    stat: (path: any) => {
      exists: boolean;
      isDir: boolean;
      size: number;
      mtime: number;
    };
    readFile: (path: any) => string;
  };
};

export const normalizeOptions = ({
  extensions = [".js"],
  extensionMap = {},
  mainFields = isESM ? ["module", "main"] : ["main"],
  conditions = isESM ? ["import"] : ["require"],
  indexName = isESM ? "" : "index",
  modulesDirectoryName = "node_modules",
  manifestFileName = "package.json",
  isCoreModule = _isCoreModule,
  path = _path,
  fs = _fs,
}: Options = {}): Options => {
  return {
    extensions,
    extensionMap,
    mainFields,
    conditions,
    indexName,
    modulesDirectoryName,
    manifestFileName,
    path,
    fs,
    isCoreModule,
  };
};
