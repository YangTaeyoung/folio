import { useSyncExternalStore } from "react";
import ko from "./locales/ko.json";
import en from "./locales/en.json";
import ja from "./locales/ja.json";
import zhCN from "./locales/zh-CN.json";
import fr from "./locales/fr.json";

export const locales = ["ko", "en", "ja", "zh-CN", "fr"] as const;
export type Locale = (typeof locales)[number];
export const localeNames: Record<Locale, string> = {
  ko: "한국어",
  en: "English",
  ja: "日本語",
  "zh-CN": "简体中文",
  fr: "Français",
};
const catalogs: Record<Locale, Record<string, string>> = {
  ko,
  en,
  ja,
  "zh-CN": zhCN,
  fr,
};
const storageKey = "folio.locale";
const listeners = new Set<() => void>();
export const isLocale = (value: unknown): value is Locale =>
  locales.includes(value as Locale);
function storedLocale(): Locale {
  try {
    const saved = localStorage.getItem(storageKey);
    return isLocale(saved) ? saved : "ko";
  } catch {
    return "ko";
  }
}
let current: Locale = storedLocale();
function applyLanguage() {
  if (typeof document !== "undefined") document.documentElement.lang = current;
}
applyLanguage();
export function getLocale() {
  return current;
}
export function setLocale(locale: Locale) {
  if (!isLocale(locale)) return;
  current = locale;
  try {
    localStorage.setItem(storageKey, locale);
  } catch {
    /* Private browsing can deny storage. */
  }
  applyLanguage();
  listeners.forEach((listener) => listener());
}
if (typeof window !== "undefined")
  window.addEventListener("storage", (event) => {
    if (event.key !== storageKey && event.key !== null) return;
    current = storedLocale();
    applyLanguage();
    listeners.forEach((listener) => listener());
  });
function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
export function useLocale() {
  return useSyncExternalStore(subscribe, getLocale, () => "ko" as Locale);
}
const reverseSources = new Map<string, string>();
for (const catalog of Object.values(catalogs))
  for (const [source, text] of Object.entries(catalog))
    reverseSources.set(text, source);
const reversePatterns: {
  source: string;
  names: string[];
  expression: RegExp;
}[] = [];
for (const catalog of Object.values(catalogs)) {
  for (const [source, text] of Object.entries(catalog)) {
    if (!/\{\w+\}/.test(text)) continue;
    const names: string[] = [];
    const parts = text.split(/(\{\w+\})/g).map((part) => {
      const name = /^\{(\w+)\}$/.exec(part);
      if (name) {
        names.push(name[1]);
        return "(.*?)";
      }
      return part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    });
    reversePatterns.push({
      source,
      names,
      expression: new RegExp("^" + parts.join("") + "$", "s"),
    });
  }
}
function resolveMessage(
  text: string,
): { source: string; params: Record<string, string> } | undefined {
  if (Object.hasOwn(ko, text)) return { source: text, params: {} };
  const exact = reverseSources.get(text);
  if (exact) return { source: exact, params: {} };
  for (const pattern of reversePatterns) {
    const match = pattern.expression.exec(text);
    if (match)
      return {
        source: pattern.source,
        params: Object.fromEntries(
          pattern.names.map((name, i) => [name, match[i + 1]]),
        ),
      };
  }
  return undefined;
}
export function localizedError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return resolveMessage(message)
    ? t(message)
    : t("요청을 처리하지 못했습니다. 잠시 후 다시 시도해 주세요.");
}
/** Source literals are stable keys. Values interpolate as plain text, never HTML. */
export function t(
  source: string,
  params: Record<string, string | number> = {},
) {
  const resolved = resolveMessage(source);
  const key = resolved?.source ?? source;
  const values = { ...resolved?.params, ...params };
  const translated = catalogs[current][key] ?? source;
  return translated.replace(/\{(\w+)\}/g, (match, key: string) =>
    Object.hasOwn(values, key)
      ? typeof values[key] === "number"
        ? number(values[key] as number)
        : String(values[key])
      : match,
  );
}
export function number(value: number, options: Intl.NumberFormatOptions = {}) {
  return new Intl.NumberFormat(current, options).format(value);
}
