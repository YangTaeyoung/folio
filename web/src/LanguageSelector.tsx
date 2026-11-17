import { Select } from "./Select";
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
      <Select
        aria-label={t("언어")}
        value={locale}
        onValueChange={(value) => {
          if (isLocale(value)) setLocale(value);
        }}
      >
        {locales.map((value) => (
          <option key={value} lang={value} value={value}>
            {localeNames[value]}
          </option>
        ))}
      </Select>
    </label>
  );
}
