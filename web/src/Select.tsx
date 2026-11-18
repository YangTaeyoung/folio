import {
  Children,
  Fragment,
  isValidElement,
  useId,
  useRef,
  useState,
  type ComponentPropsWithoutRef,
  type ReactNode,
} from "react";
import * as RadixSelect from "@radix-ui/react-select";
import { Check, ChevronDown, ChevronUp } from "lucide-react";
import { ServiceIcon, type ServiceId } from "./ServiceIcon";
import { t, useLocale } from "./i18n";
import "./Select.css";

type Option = {
  value: string;
  label: string;
  disabled?: boolean;
  service?: ServiceId;
  lang?: string;
};
type Props = Omit<
  ComponentPropsWithoutRef<"button">,
  "value" | "defaultValue" | "onChange" | "children"
> & {
  value?: string | number;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  children: ReactNode;
  required?: boolean;
  form?: string;
  triggerIcon?: ReactNode;
};
function text(node: ReactNode): string {
  return Children.toArray(node)
    .map((child) =>
      isValidElement<{ children?: ReactNode }>(child)
        ? text(child.props.children)
        : String(child),
    )
    .join("");
}
function options(children: ReactNode): Option[] {
  const result: Option[] = [];
  Children.forEach(children, (child) => {
    if (
      !isValidElement<{
        value?: string | number;
        children?: ReactNode;
        disabled?: boolean;
        "data-service"?: ServiceId;
        lang?: string;
      }>(child)
    )
      return;
    if (child.type === Fragment) result.push(...options(child.props.children));
    else if (child.type === "option")
      result.push({
        value: String(child.props.value ?? text(child.props.children)),
        label: text(child.props.children),
        disabled: child.props.disabled,
        service: child.props["data-service"],
        lang: child.props.lang,
      });
  });
  return result;
}
/** Existing option declarations stay data; only Radix renders the interactive UI. */
export function Select({
  value,
  defaultValue,
  onValueChange,
  children,
  name,
  required,
  disabled,
  form,
  className = "",
  triggerIcon,
  ...trigger
}: Props) {
  useLocale();
  const [invalid, setInvalid] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const errorId = useId();
  const items = options(children);
  // Uploaded filenames can be arbitrary strings; never reserve a real option's
  // value for the empty (whole-version) choice.
  let empty = "__folio_empty_option__";
  while (items.some((item) => item.value === empty)) empty += "_";
  const encode = (option: string) =>
    option === "" && !required ? empty : option;
  const selected = items.find(
    (item) => item.value === String(value ?? defaultValue ?? items[0]?.value),
  );
  return (
    <span
      className="select-root"
      onInvalidCapture={(event) => {
        // Radix supplies native constraint validation via a visually hidden
        // select. Keep its validation, but put feedback/focus on the visible UI.
        event.preventDefault();
        setInvalid(true);
        triggerRef.current?.focus();
      }}
    >
      <RadixSelect.Root
        value={value === undefined ? undefined : encode(String(value))}
        defaultValue={encode(defaultValue ?? items[0]?.value ?? "")}
        onValueChange={(next) => {
          setInvalid(false);
          onValueChange?.(next === empty ? "" : next);
        }}
        name={name}
        required={required}
        disabled={disabled}
        form={form}
      >
        <RadixSelect.Trigger
          {...trigger}
          ref={triggerRef}
          aria-invalid={invalid || trigger["aria-invalid"]}
          aria-describedby={
            [trigger["aria-describedby"], invalid ? errorId : undefined]
              .filter(Boolean)
              .join(" ") || undefined
          }
          className={`select-control ${triggerIcon ? "select-icon-only" : ""} ${className}`}
          data-value={value ?? defaultValue ?? items[0]?.value}
        >
          {triggerIcon}
          {selected?.service && !triggerIcon && (
            <ServiceIcon service={selected.service} size={18} />
          )}
          <span className={triggerIcon ? "sr-only" : "select-value"}>
            <RadixSelect.Value
              placeholder={items.find((item) => item.value === "")?.label}
            />
          </span>
          {!triggerIcon && (
            <RadixSelect.Icon className="select-chevron">
              <ChevronDown size={16} />
            </RadixSelect.Icon>
          )}
        </RadixSelect.Trigger>
        <RadixSelect.Portal>
          <RadixSelect.Content
            data-folio-select-content
            className="select-content"
            position="popper"
            sideOffset={6}
            collisionPadding={12}
            onEscapeKeyDown={(e) => e.stopPropagation()}
          >
            <RadixSelect.ScrollUpButton className="select-scroll">
              <ChevronUp size={16} />
            </RadixSelect.ScrollUpButton>
            <RadixSelect.Viewport className="select-viewport">
              {items.map((item) => (
                <RadixSelect.Item
                  className="select-option"
                  key={item.value}
                  value={item.value === "" ? empty : item.value}
                  disabled={item.disabled || (required && item.value === "")}
                  textValue={item.label}
                  lang={item.lang}
                  data-value={item.value}
                >
                  {item.service && (
                    <ServiceIcon service={item.service} size={20} />
                  )}
                  <RadixSelect.ItemText>{item.label}</RadixSelect.ItemText>
                  <RadixSelect.ItemIndicator className="select-check">
                    <Check size={16} />
                  </RadixSelect.ItemIndicator>
                </RadixSelect.Item>
              ))}
            </RadixSelect.Viewport>
            <RadixSelect.ScrollDownButton className="select-scroll">
              <ChevronDown size={16} />
            </RadixSelect.ScrollDownButton>
          </RadixSelect.Content>
        </RadixSelect.Portal>
      </RadixSelect.Root>
      {invalid && (
        <span id={errorId} className="select-error" role="alert">
          {t("항목을 선택해 주세요.")}
        </span>
      )}
    </span>
  );
}
