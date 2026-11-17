import { t as translate, useLocale, localizedError } from "./i18n";
import {
  cloneElement,
  isValidElement,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import type { ReactElement, ReactNode } from "react";
import {
  AlertCircle,
  Check,
  CheckCircle2,
  Copy,
  Loader2,
  X,
} from "lucide-react";
import { copy } from "./api";

export function Spinner() {
  useLocale();
  return (
    <Loader2 className="spin" size={18} aria-label={translate("불러오는 중")} />
  );
}
export function ErrorBox({ error }: { error: unknown }) {
  useLocale();
  return error ? (
    <div role="alert" className="error-box">
      <AlertCircle size={18} />
      <span>{localizedError(error)}</span>
    </div>
  ) : null;
}
export function Notice({ children }: { children: ReactNode }) {
  useLocale();
  return (
    <div className="notice">
      <CheckCircle2 size={17} />
      <span>
        {typeof children === "string" ? translate(children) : children}
      </span>
    </div>
  );
}
export function Field({
  label,
  children,
  hint,
}: {
  label: string;
  children: ReactNode;
  hint?: string;
}) {
  useLocale();
  const id = useId();
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {isValidElement(children)
        ? cloneElement(children as ReactElement<Record<string, unknown>>, {
            id,
            "aria-describedby": hint ? `${id}-hint` : undefined,
          })
        : children}
      {hint && <small id={`${id}-hint`}>{hint}</small>}
    </div>
  );
}
export function CopyButton({
  text,
  label = translate("복사"),
  onCopied,
}: {
  text: string;
  label?: string;
  onCopied?: () => void;
}) {
  useLocale();
  const [done, setDone] = useState(false);
  const [error, setError] = useState("");
  return (
    <>
      <button
        type="button"
        className="button secondary compact"
        onClick={async () => {
          try {
            await copy(text);
            setDone(true);
            onCopied?.();
            setTimeout(() => setDone(false), 1800);
          } catch (e) {
            setError((e as Error).message);
          }
        }}
      >
        {done ? <Check size={15} /> : <Copy size={15} />}{" "}
        {done ? translate("복사됨") : label}
      </button>
      {error && (
        <span role="alert" className="inline-error">
          {localizedError(error)}
        </span>
      )}
    </>
  );
}
export function Avatar({
  name,
  src,
  className = "",
}: {
  name: string;
  src?: string;
  className?: string;
}) {
  useLocale();
  const [failed, setFailed] = useState("");
  return (
    <span className={`avatar ${className}`} aria-hidden="true">
      {src && failed !== src ? (
        <img src={src} alt="" onError={() => setFailed(src)} />
      ) : (
        name.trim().slice(0, 2).toUpperCase()
      )}
    </span>
  );
}
export function Modal({
  title,
  children,
  onClose,
  wide = false,
  drawer = false,
  className = "",
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
  drawer?: boolean;
  className?: string;
}) {
  useLocale();
  const ref = useRef<HTMLDivElement>(null);
  const close = useRef(onClose);
  close.current = onClose;
  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const el = ref.current;
    const selectors =
      'button:not(:disabled),a[href],input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]';
    const first = el?.querySelector<HTMLElement>(selectors);
    first?.focus();
    function key(e: KeyboardEvent) {
      // A portalled Select owns Escape/Tab until its menu closes. Do not close
      // the containing dialog or move focus behind that active menu.
      if (
        e.defaultPrevented ||
        document.querySelector('[data-folio-select-content][data-state="open"]')
      )
        return;
      if (e.key === "Escape") {
        e.preventDefault();
        close.current();
      }
      if (e.key === "Tab" && el) {
        const list = [...el.querySelectorAll<HTMLElement>(selectors)].filter(
          (n) => n.offsetParent !== null,
        );
        if (!list.length) {
          e.preventDefault();
          el.focus();
          return;
        }
        const a = list[0],
          b = list.at(-1)!;
        if (e.shiftKey && document.activeElement === a) {
          e.preventDefault();
          b.focus();
        } else if (!e.shiftKey && document.activeElement === b) {
          e.preventDefault();
          a.focus();
        }
      }
    }
    document.addEventListener("keydown", key);
    return () => {
      document.body.style.overflow = overflow;
      document.removeEventListener("keydown", key);
      prev?.focus();
    };
  }, []);
  return (
    <div
      className={`modal-backdrop ${drawer ? "drawer-backdrop" : ""}`}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={ref}
        className={`${drawer ? "drawer" : "modal"} ${wide ? "wide" : ""} ${className}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
      >
        <header className="dialog-header">
          <h2>{title}</h2>
          <button
            className="icon-button"
            aria-label={translate("닫기")}
            onClick={onClose}
          >
            <X size={21} />
          </button>
        </header>
        {children}
      </div>
    </div>
  );
}
export function Empty({
  icon,
  heading,
  description,
  action,
}: {
  icon: ReactNode;
  heading: string;
  description: string;
  action?: ReactNode;
}) {
  useLocale();
  return (
    <div className="empty">
      <div className="empty-symbol">{icon}</div>
      <h2>{heading}</h2>
      <p>{description}</p>
      {action}
    </div>
  );
}
export function useAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  return {
    busy,
    error,
    setError,
    run: async <T,>(fn: () => Promise<T>): Promise<T | undefined> => {
      setBusy(true);
      setError(null);
      try {
        return await fn();
      } catch (e) {
        setError(e);
        return undefined;
      } finally {
        setBusy(false);
      }
    },
  };
}
