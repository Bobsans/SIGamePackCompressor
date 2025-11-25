import { type Plugin } from "vite";

export default (replacements: Record<string, string>): Plugin => ({
  name: "vite-replace-plugin",
  enforce: "pre",
  transform: (code) => {
    if (code.match(/<!\[.*]!>/g)) {
      return Object.entries(replacements).reduce((result, [key, val]) => result.replace(`<![${key}]!>`, val), code);
    }
    return code;
  }
});
