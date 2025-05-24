export type Specifier = {
  proto?: string;
  scope?: string;
  pkg?: string;
  name?: string;
  path?: string;
};

const regexp = /^(?:@(\w[\w-.]*)\/)?(\w[\w-.]*)(\/(.*))?/;

/**
 * Parse a input string into a Specifier.
 *
 * input can be a npm package name with optional proto, subpath.
 */
export const parseSpecifier = (input: string): Specifier | null => {
  if (!input) {
    return null;
  }
  const sp = input.split(":");
  const path = sp.pop();

  const match = path.match(regexp);

  console.log(match);
  if (!match || !match[0] || !match[2]) {
    return null;
  }

  const scope = match[1];
  const pkg = match[2];
  const result: Specifier = {
    name: scope ? `@${scope}/${pkg}` : pkg,
    scope,
    pkg,
    proto: sp.pop(),
    path: match[4],
  };
  return result;
};
