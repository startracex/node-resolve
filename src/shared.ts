import { readFileSync, statSync } from "node:fs";
import { dirname as dir, join, isAbsolute as isAbs } from "node:path";
import { builtinModules } from "node:module";

// replace by rollup-plugin-cjs-shim
const isESM = typeof import.meta !== "undefined";

const coreModules = new Set(builtinModules);

const _isCoreModule = (id: string) => {
  if (id.startsWith("node:")) {
    return true;
  }
  if (coreModules.has(id)) {
    return true;
  }
  const slash = id.indexOf("/");
  return slash !== -1 && coreModules.has(id.slice(0, slash));
};

const _fs = {
  stat: (path: string) => {
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
  readFile: (path: string) => {
    return readFileSync(path, "utf-8");
  },
};

const _path = {
  dir,
  join,
  isAbs,
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
    isAbs: (path: string) => boolean;
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
  conditions = isESM ? ["node", "import"] : ["node", "require"],
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
