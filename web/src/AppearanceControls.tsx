import { Select } from "./Select";
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
  return (
    <div className="appearance-controls">
      <LanguageSelector />
      <label className="theme-selector">
        <Icon size={16} aria-hidden="true" />
        <Select
          aria-label={t("화면 테마")}
          value={theme}
          onValueChange={(value) => window.folioTheme.set(value as Theme)}
        >
          <option value="system">{t("시스템 설정")}</option>
          <option value="light">{t("라이트 모드")}</option>
          <option value="dark">{t("다크 모드")}</option>
        </Select>
      </label>
    </div>
  );
}
