import { useSyncExternalStore } from "react";
import { Monitor, Moon, Sun } from "lucide-react";
import { LanguageSelector } from "./LanguageSelector";
import { t, useLocale } from "./i18n";
import "./AppearanceControls.css";

type Theme = "system" | "light" | "dark";
declare global {
  interface Window {
    folioTheme: {
      get: () => Theme;
      set: (value: Theme) => void;
      subscribe: (listener: () => void) => () => void;
    };
  }
}
export function AppearanceControls() {
  useLocale();
  const theme = useSyncExternalStore(
    window.folioTheme.subscribe,
    window.folioTheme.get,
  );
  const Icon = theme === "system" ? Monitor : theme === "dark" ? Moon : Sun;
  const themes: Theme[] = ["system", "light", "dark"];
  const next = themes[(themes.indexOf(theme) + 1) % themes.length];
  const labels = {
    system: t("시스템 설정"),
    light: t("라이트 모드"),
    dark: t("다크 모드"),
  };
  return (
    <div className="appearance-controls">
      <LanguageSelector />
      <button
        type="button"
        className="theme-toggle icon-button"
        aria-label={t("화면 테마")}
        title={`${labels[theme]} → ${labels[next]}`}
        data-preference={theme}
        onClick={() => window.folioTheme.set(next)}
      >
        <Icon size={17} aria-hidden="true" />
        <span className="sr-only">{labels[theme]}</span>
      </button>
    </div>
  );
}
