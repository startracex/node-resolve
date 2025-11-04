import { readFileSync, statSync } from "node:fs";
import { dirname as dir, join } from "node:path";
import { builtinModules } from "node:module";

const isNodeProto = (id: string) => id.startsWith("node:");

const coreModuleSet = new Set(
  builtinModules
    .filter((id) => !isNodeProto(id))
    .map((id) => {
      const index = id.indexOf("/");
      return index === -1 ? id : id.substring(0, index);
    }),
);

const _isCoreModule = (id: string) => isNodeProto(id) || coreModuleSet.has(id);

const _fs: {
  stat: (path) => {
    exists: boolean;
    isDir: boolean;
    size: number;
    mtime: number;
  };
  readFile: (path) => string;
} = {
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

const _path: {
  dir: (path: string) => string;
  join: (...paths: string[]) => string;
} = {
  dir,
  join,
};

const _default = "default";

export type Options = {
  extensions?: string[];
  extensionMap?: {};
  mainFields?: string[];
  conditions?: string[];
  indexName?: string;
  modulesDirectoryName?: string;
  manifestFileName?: string;
  isCoreModule?: (id: any) => boolean;
  path?: typeof _path;
  fs?: typeof _fs;
};

export const normalizeOptions = ({
  extensions = [".js"],
  extensionMap = {},
  mainFields = ["main"],
  conditions = [_default],
  indexName = "index",
  modulesDirectoryName = "node_modules",
  manifestFileName = "package.json",
  isCoreModule = _isCoreModule,
  path = _path,
  fs = _fs,
}: Options = {}): Options => {
  if (!conditions.includes(_default)) {
    conditions.push(_default);
  }
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
