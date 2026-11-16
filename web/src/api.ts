import { getLocale, number, t } from "./i18n";
let csrf = "";
export function setCSRF(value: string) {
  csrf = value;
}
export class APIError extends Error {
  status: number;
  code: string;
  constructor(message: string, status: number, code: string) {
    super(message);
    Object.defineProperty(this, "message", {
      get: () => apiErrorMessage(code, status, message),
    });
    this.status = status;
    this.code = code;
  }
}
export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (
    options.body &&
    !(options.body instanceof FormData) &&
    !headers.has("Content-Type")
  )
    headers.set(
      "Content-Type",
      options.body instanceof Blob
        ? "application/octet-stream"
        : "application/json",
    );
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
  if (!Number.isFinite(n) || n < 0) return "—";
  if (n < 1024) return `${number(n)} B`;
  const unit = Math.min(3, Math.floor(Math.log(n) / Math.log(1024)));
  return `${number(n / 1024 ** unit, { minimumFractionDigits: unit === 1 ? 0 : 1, maximumFractionDigits: unit === 1 ? 0 : 1 })} ${["B", "KB", "MB", "GB"][unit]}`;
}
export function date(value: string) {
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? "—"
    : new Intl.DateTimeFormat(getLocale(), {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      }).format(d);
}
export function relative(value: string) {
  const delta = Date.now() - new Date(value).getTime();
  if (!Number.isFinite(delta)) return "—";
  if (Math.abs(delta) < 60000) return t("방금 전");
  const [divisor, unit] =
    Math.abs(delta) < 3600000
      ? ([60000, "minute"] as const)
      : Math.abs(delta) < 86400000
        ? ([3600000, "hour"] as const)
        : ([86400000, "day"] as const);
  return new Intl.RelativeTimeFormat(getLocale(), { numeric: "always" }).format(
    -Math.trunc(delta / divisor),
    unit,
  );
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
  if (!ok)
    throw new Error(t("복사하지 못했습니다. 링크를 선택해 복사해 주세요."));
}

const errorSources: Record<string, string> = {
  review_version_required: "검토할 버전을 선택해 주세요.",
  version_required: "버전을 선택해 주세요.",
  base_version_required: "현재 공식 버전을 확인하고 다시 시도해 주세요.",
  version_conflict:
    "공식 버전이 변경되었습니다. 새로고침 후 다시 게시해 주세요.",
  review_not_approved: "이 버전의 승인이 완료되어야 게시할 수 있습니다.",
  thread_resolved: "답글을 남기려면 먼저 대화를 다시 열어 주세요.",
  comment_deleted: "삭제된 댓글은 수정할 수 없습니다.",
  stale_collection: "컬렉션이 변경되었습니다. 새로고침 후 다시 저장해 주세요.",
  invalid_backup:
    "백업 파일과 비밀번호를 확인해 주세요. 백업을 검증하거나 복원하지 못했습니다.",
  backup_too_large: "백업 크기 한도를 초과했습니다.",
  storage_busy:
    "업로드가 진행 중입니다. 완료된 후 백업 작업을 다시 시도해 주세요.",
  artifact_busy:
    "이 아티팩트의 업로드가 진행 중입니다. 완료된 후 다시 시도해 주세요.",
  already_deleted: "이미 휴지통에 있는 아티팩트입니다.",
  invalid_credentials: "이메일 또는 비밀번호가 올바르지 않습니다.",
  invalid_password: "비밀번호가 올바르지 않습니다.",
  weak_password: "비밀번호는 12~72바이트로 설정해 주세요.",
  invalid_email: "올바른 이메일 주소를 입력해 주세요.",
  unauthorized: "로그인이 필요합니다.",
  forbidden: "이 작업을 수행할 권한이 없습니다.",
  insufficient_scope: "API 토큰에 이 작업에 필요한 권한이 없습니다.",
  csrf_failed: "세션을 확인하지 못했습니다. 페이지를 새로고침해 주세요.",
  invalid_origin:
    "요청 주소를 확인하지 못했습니다. 페이지를 새로고침해 주세요.",
  not_found: "요청한 항목을 찾을 수 없습니다.",
  rate_limited: "요청이 너무 많습니다. 잠시 후 다시 시도해 주세요.",
  quota_exceeded: "저장 용량 한도를 초과했습니다. 관리자에게 문의해 주세요.",
  upload_too_large: "업로드 크기 한도를 초과했습니다.",
  storage_failed:
    "저장소 작업을 완료하지 못했습니다. 연결 설정을 확인해 주세요.",
  storage_unavailable:
    "저장소를 사용할 수 없습니다. 연결 상태를 확인해 주세요.",
  upload_failed: "업로드하지 못했습니다. 파일과 연결 상태를 확인해 주세요.",
  storage_cleanup_failed: "저장소 정리를 완료하지 못했습니다.",
  email_exists: "이미 등록된 이메일입니다.",
  slug_exists: "이미 사용 중인 아티팩트 주소입니다.",
  hostname_exists: "이미 연결된 호스트 이름입니다.",
  last_admin: "마지막 관리자는 중지하거나 역할을 변경할 수 없습니다.",
  google_only_account: "이 계정은 Google 로그인을 사용합니다.",
  google_config_required: "먼저 Google 로그인을 설정해 주세요.",
  google_account_unavailable:
    "이 Google 계정으로 로그인할 수 없습니다. 관리자에게 문의해 주세요.",
  password_change_required: "계속하려면 비밀번호를 변경해 주세요.",
  already_configured: "초기 설정이 이미 완료되었습니다.",
  invalid_bootstrap_token: "설치 키가 올바르지 않습니다.",
  required_field: "필수 항목을 입력해 주세요.",
  members_required: "공유할 구성원을 선택해 주세요.",
  index_required: "시작 파일 index.html이 필요합니다.",
  artifact_deleted: "휴지통에 있는 아티팩트입니다. 먼저 복원해 주세요.",
  stale_version: "버전이 변경되었습니다. 새로고침 후 다시 시도해 주세요.",
  backend_referenced: "이 저장소를 사용하는 버전이 남아 있습니다.",
  default_backend: "기본 저장소는 제거할 수 없습니다.",
  backend_busy:
    "저장소에서 다른 작업이 진행 중입니다. 잠시 후 다시 시도해 주세요.",
  dns_failed: "DNS 작업에 실패했습니다. DNS 설정을 확인해 주세요.",
  dns_collision: "기존 DNS 레코드와 충돌합니다. 레코드를 확인해 주세요.",
  dns_not_connected: "먼저 DNS 서비스를 연결해 주세요.",
  dns_unavailable: "DNS 서비스를 사용할 수 없습니다.",
  zone_not_found: "연결할 DNS 영역을 찾을 수 없습니다.",
  zone_mismatch: "호스트 이름과 DNS 영역이 일치하지 않습니다.",
  tunnel_not_configured: "먼저 전용 터널을 구성해 주세요.",
  manual_dns_required: "DNS 레코드를 직접 등록해 주세요.",
  manual_dns_cleanup_required:
    "DNS 서비스에서 기존 레코드를 직접 제거해 주세요.",
  hostname_reserved: "이 호스트 이름은 예약되어 있습니다.",
  unbound_hostname: "이 아티팩트에 연결되지 않은 호스트 이름입니다.",
};
export function apiErrorMessage(code: string, status: number, fallback = "") {
  if (code === "weak_password" && fallback.startsWith("Backup password"))
    return t("백업 비밀번호는 12~256바이트로 설정해 주세요.");
  if (errorSources[code]) return t(errorSources[code]);
  if (
    code.startsWith("invalid_") ||
    ["missing_file", "extra_file", "metadata_required"].includes(code)
  )
    return t("입력값 또는 파일을 확인해 주세요.");
  if (status === 401) return t("로그인이 필요합니다.");
  if (status === 403) return t("이 작업을 수행할 권한이 없습니다.");
  if (status === 404) return t("요청한 항목을 찾을 수 없습니다.");
  // Backend error text is not a translation key or trusted UI content.
  // Known source literals can be safely translated; unknown text gets a localized recovery message.
  if (
    fallback &&
    /[가-힣]/.test(fallback) &&
    fallback === "요청을 처리하지 못했습니다."
  )
    return t(fallback);
  return t("요청을 처리하지 못했습니다. 잠시 후 다시 시도해 주세요.");
}
