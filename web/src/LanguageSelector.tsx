import { Languages } from "lucide-react";
import {
  isLocale,
  localeNames,
  locales,
  setLocale,
  t,
  useLocale,
} from "./i18n";
import "./LanguageSelector.css";
export function LanguageSelector() {
  const locale = useLocale();
  return (
    <label className="language-selector">
      <Languages size={16} aria-hidden="true" />
      <select
        aria-label={t("언어")}
        value={locale}
        onChange={(event) => {
          if (isLocale(event.target.value)) setLocale(event.target.value);
        }}
      >
        {locales.map((value) => (
          <option key={value} lang={value} value={value}>
            {localeNames[value]}
          </option>
        ))}
      </select>
    </label>
  );
}
