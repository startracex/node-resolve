import { SubpathResolver } from "./subpath-resolver.js";
import { parseSpecifier, Specifier } from "./specifier.js";

export class ModuleResolver extends SubpathResolver {
  extensions?: string[];
  modulesDirectoryName: string;
  manifestFileName: string;
  mainFields: string[];
  isCoreModule: (name: string) => boolean;
  path: {
    dirname: (path: string) => string;
    join: (...paths: string[]) => string;
  };
  fs: {
    statSync: (path: string) => {
      isFile: () => boolean;
      isDirectory: () => boolean;
    };
    readFileSync: (path: string) => string | {};
  };
  index: string;

  constructor({
    extensions,
    isCoreModule,
    modulesDirectoryName = "node_modules",
    manifestFileName = "package.json",
    mainFields = ["main"],
    index = "index",
    path,
    fs,
    ...args
  }: Partial<ModuleResolver> = {}) {
    super(args);
    this.extensions = extensions;
    this.isCoreModule = isCoreModule;
    this.modulesDirectoryName = modulesDirectoryName;
    this.manifestFileName = manifestFileName;
    this.mainFields = mainFields;
    this.path = path;
    this.fs = fs;
    this.index = index;
  }

  modulesPaths(start?: string, name = ""): string[] {
    const paths: string[] = [];

    if (!start) {
      return paths;
    }

    for (;;) {
      paths.push(this.path.join(start, this.modulesDirectoryName, name));
      const parent = this.path.dirname(start);
      if (parent === start) {
        break;
      }
      start = parent;
    }

    return paths;
  }

  resolve(path: string, base: string): string | undefined {
    if (path.startsWith("#")) {
      const paths = this.resolveImports(path);
      if (paths) {
        for (const p of paths) {
          const rd = this.resolveFileOrDir(p);
          if (rd) {
            return rd;
          }
        }
      }
      return;
    }

    const spec = parseSpecifier(path);

    if (spec?.name) {
      if (this.isCoreModule?.(spec.name)) {
        return path;
      }
      return this.resolveModuleSpecifier(spec, base);
    }

    return this.resolveFileOrDir(this.path.join(base, path));
  }

  resolveModuleSpecifier(
    { name, path }: Specifier,
    base?: string,
  ): string | undefined {
    const dirs = this.modulesPaths(base, name);
    for (const dir of dirs) {
      const stat = this.stat(dir);
      if (stat?.isDirectory()) {
        const rd = this.resolveDir(dir, path);
        if (rd) {
          return rd;
        }
      }
    }
  }

  protected resolveFile(filePath: string): string | undefined {
    const candidates = [
      filePath,
      ...(this.extensions || []).map((ext) => filePath + ext),
    ];
    for (const file of candidates) {
      const stat = this.stat(file);
      if (!stat || stat.isDirectory()) {
        continue;
      }
      if (stat.isFile()) {
        return file;
      }
    }
  }

  protected resolveDir(dirPath: string, entry?: string): string | undefined {
    const packageJsonPath = this.path.join(dirPath, this.manifestFileName);
    const stat = this.stat(packageJsonPath);
    if (!stat?.isFile()) {
      return this.resolveFile(this.path.join(dirPath, this.index));
    }

    const pkg = this.readJSON(packageJsonPath);
    if (pkg.exports) {
      const exportsMatchArray = new SubpathResolver({
        exports: pkg.exports,
        conditions: this.conditions,
      }).resolveExports(entry);
      if (exportsMatchArray) {
        for (let match of exportsMatchArray) {
          match = this.path.join(dirPath, match);
          const stat = this.stat(match);
          if (stat?.isFile()) {
            return match;
          }
        }
      }
      return;
    }
    if (!entry) {
      for (const field of this.mainFields) {
        let main = pkg[field];
        if (!main) {
          continue;
        }
        main = this.path.join(dirPath, main);
        const stat = this.stat(main);
        if (stat?.isFile()) {
          return main;
        }
      }
      return;
    }
    const subPath = this.path.join(dirPath, entry);
    return this.resolveFileOrDir(subPath);
  }

  protected resolveFileOrDir(subPath: string, entry?: string): string | undefined {
    return this.resolveFile(subPath) || this.resolveDir(subPath, entry);
  }

  protected readJSON(fullPath: string): Record<string, any> {
    return JSON.parse(this.fs.readFileSync(fullPath).toString());
  }

  protected stat(path: string): {
    isFile(): boolean;
    isDirectory(): boolean;
  } | null {
    try {
      return this.fs.statSync(path);
    } catch (e) {
      return null;
    }
  }
}
