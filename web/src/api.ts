let csrf = "";
export function setCSRF(value: string) {
  csrf = value;
}
export class APIError extends Error {
  status: number;
  code: string;
  constructor(message: string, status: number, code: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}
export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body && !(options.body instanceof FormData))
    headers.set("Content-Type", "application/json");
  if (
    options.method &&
    !["GET", "HEAD"].includes(options.method.toUpperCase()) &&
    csrf
  )
    headers.set("X-CSRF-Token", csrf);
  const response = await fetch(path, {
    ...options,
    headers,
    credentials: "same-origin",
  });
  if (response.status === 204) return undefined as T;
  const data = await response.json().catch(() => ({}));
  if (response.status === 401 && path !== "/api/auth/login")
    window.dispatchEvent(new Event("folio:session-expired"));
  if (!response.ok)
    throw new APIError(
      data.error?.message || "요청을 처리하지 못했습니다.",
      response.status,
      data.error?.code || "request_failed",
    );
  return data as T;
}
export const json = (method: string, body?: unknown): RequestInit => ({
  method,
  body: body === undefined ? undefined : JSON.stringify(body),
});
export function bytes(n: number) {
  if (n < 1024) return `${n} B`;
  const unit = Math.min(3, Math.floor(Math.log(n) / Math.log(1024)));
  return `${(n / 1024 ** unit).toFixed(unit === 1 ? 0 : 1)} ${["B", "KB", "MB", "GB"][unit]}`;
}
export function date(value: string) {
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? "—"
    : new Intl.DateTimeFormat("ko-KR", {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      }).format(d);
}
export function relative(value: string) {
  const delta = Date.now() - new Date(value).getTime();
  if (delta < 60000) return "방금 전";
  if (delta < 3600000) return `${Math.floor(delta / 60000)}분 전`;
  if (delta < 86400000) return `${Math.floor(delta / 3600000)}시간 전`;
  return `${Math.floor(delta / 86400000)}일 전`;
}
export async function copy(text: string) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const el = document.createElement("textarea");
  el.value = text;
  el.style.position = "fixed";
  el.style.opacity = "0";
  document.body.append(el);
  el.select();
  const ok = document.execCommand("copy");
  el.remove();
  if (!ok) throw new Error("복사하지 못했습니다. 링크를 선택해 복사해 주세요.");
}
