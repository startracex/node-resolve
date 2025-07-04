type Value = string | string[] | { [key: string]: Value };

const normalizeMapping = (m: Value): Record<string, Value> => {
  if (!m) {
    return {};
  }
  if (typeof m === "string" || Array.isArray(m)) {
    return { ".": m };
  }
  return m;
};

const findWildcardMatch = (
  mapping: Record<string, Value>,
  input: string,
): { key: string; replacement: string } | null => {
  let bestMatch: { key: string; length: number } | null = null;
  let replacement = "";

  for (const key in mapping) {
    if (key.endsWith("/*")) {
      const prefix = key.slice(0, -1);
      if (input.startsWith(prefix)) {
        const matchLength = prefix.length;
        if (!bestMatch || matchLength > bestMatch.length) {
          bestMatch = { key, length: matchLength };
          replacement = input.slice(prefix.length);
        }
      }
    } else if (key.includes("*")) {
      const [before, after] = key.split("*");
      if (input.startsWith(before) && input.endsWith(after)) {
        const matchLength = before.length + after.length;
        if (!bestMatch || matchLength > bestMatch.length) {
          bestMatch = { key, length: matchLength };
          replacement = input.slice(before.length, input.length - after.length);
        }
      }
    } else if (key.endsWith("/") && input.startsWith(key)) {
      const matchLength = key.length;
      if (!bestMatch || matchLength > bestMatch.length) {
        bestMatch = { key, length: matchLength };
        replacement = input.slice(key.length);
      }
    }
  }

  return bestMatch ? { key: bestMatch.key, replacement } : null;
};

const resolveMappingValue = (value: Value | undefined, conditions: string[]): string[] | null => {
  if (!value) return null;

  if (typeof value === "string") {
    return [value];
  }

  if (Array.isArray(value)) {
    return value.flatMap((v) => resolveMappingValue(v, conditions) || []);
  }

  for (const [condition, subValue] of Object.entries(value)) {
    if (conditions.includes(condition)) {
      return resolveMappingValue(subValue, conditions);
    }
  }

  return null;
};

export const resolveMapping = (
  mapping: Record<string, Value>,
  conditions: string[],
  input: string,
): string[] | null => {
  let value = mapping[input];
  let wildcardMatch: { key: string; replacement: string } | null = null;

  if (value === undefined) {
    wildcardMatch = findWildcardMatch(mapping, input);
    if (wildcardMatch) {
      value = mapping[wildcardMatch.key];
    }
  }

  const resolved = resolveMappingValue(value, conditions);
  if (!resolved) {
    return null;
  }

  if (!wildcardMatch) {
    return resolved;
  }

  return resolved.map((r) => {
    if (r.includes("*")) {
      return r.replace("*", wildcardMatch.replacement);
    }
    if (r.endsWith("/")) {
      return r + wildcardMatch.replacement;
    }
    return r;
  });
};

const subpathPrefix = "./";

const normalizeEntry = (entry: string) => {
  if (!entry || entry === "." || entry === subpathPrefix) {
    return ".";
  }
  if (entry.startsWith(subpathPrefix)) {
    return entry;
  }
  return subpathPrefix + entry;
};

export class SubpathResolver {
  conditions: string[];
  exports: Record<string, Value>;
  imports: Record<string, Value>;

  constructor({
    exports,
    imports,
    conditions = ["default"],
  }: {
    exports?: Value;
    imports?: Record<string, Value>;
    conditions?: string[];
  }) {
    this.imports = imports;
    this.exports = normalizeMapping(exports);
    this.conditions = conditions;
  }

  resolveExports(entry?: string): string[] {
    return resolveMapping(this.exports, this.conditions, normalizeEntry(entry));
  }

  resolveImports(entry: string): string[] | null {
    if (!this.imports) {
      return null;
    }
    return resolveMapping(this.imports, this.conditions, entry);
  }
}
