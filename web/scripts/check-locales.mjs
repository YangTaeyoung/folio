import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join, dirname } from "node:path";
import assert from "node:assert/strict";
const root = join(dirname(fileURLToPath(import.meta.url)), "..", "src");
export const localeIDs = ["ko", "en", "ja", "zh-CN", "fr"];
export function loadCatalogs() {
  return Object.fromEntries(
    localeIDs.map((locale) => [
      locale,
      JSON.parse(readFileSync(join(root, "locales", `${locale}.json`), "utf8")),
    ]),
  );
}
export function sourceKeys() {
  const keys = new Set();
  for (const file of readdirSync(root).filter((name) =>
    /\.(tsx|ts)$/.test(name),
  )) {
    const source = readFileSync(join(root, file), "utf8");
    // Korean source literals are catalog keys, including error-code table values.
    // Quoted user content is never produced by these source modules.
    const literals = source.matchAll(/"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'/gs);
    for (const match of literals) {
      const literal = match[0];
      if (!/[가-힣]/.test(literal)) continue;
      const value = literal.startsWith('"')
        ? JSON.parse(literal)
        : literal.slice(1, -1).replace(/\\'/g, "'").replace(/\\n/g, "\n");
      keys.add(value);
    }
  }
  return [...keys].sort();
}
export function checkCatalogs() {
  const catalogs = loadCatalogs();
  const keys = Object.keys(catalogs.ko).sort();
  for (const locale of localeIDs) {
    assert.deepEqual(
      Object.keys(catalogs[locale]).sort(),
      keys,
      `${locale}: catalog keys differ`,
    );
    for (const key of keys) {
      const value = catalogs[locale][key];
      assert.equal(
        typeof value,
        "string",
        `${locale}: invalid value for ${key}`,
      );
      assert.ok(value.trim().length, `${locale}: empty translation for ${key}`);
      if (locale !== "ko")
        assert.ok(
          !/[가-힣]/.test(value),
          `${locale}: untranslated Korean: ${key}`,
        );
      assert.deepEqual(
        [...key.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort(),
        [...value.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort(),
        `${locale}: interpolation mismatch for ${key}`,
      );
    }
  }
  const missing = sourceKeys().filter(
    (key) => !Object.hasOwn(catalogs.ko, key),
  );
  assert.deepEqual(
    missing,
    [],
    `Missing source keys:\n${JSON.stringify(missing, null, 2)}`,
  );
  return { locales: localeIDs.length, keys: keys.length };
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  if (process.argv.includes("--missing")) {
    const catalogs = loadCatalogs();
    console.log(
      JSON.stringify(
        sourceKeys().filter((key) => !Object.hasOwn(catalogs.ko, key)),
        null,
        2,
      ),
    );
  } else console.log(checkCatalogs());
}
