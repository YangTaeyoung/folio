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
    <div className="language-selector">
      <Select
        aria-label={t("언어")}
        title={`${t("언어")}: ${localeNames[locale]}`}
        triggerIcon={<Languages size={17} aria-hidden="true" />}
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
    </div>
  );
}
