import { useEffect, useMemo, useState } from "react";
import { ChevronDown, Download, RefreshCw, Search } from "lucide-react";
import { api, bytes, date } from "./api";
import { t, useLocale } from "./i18n";
import { Select } from "./Select";
import { ErrorBox, Spinner } from "./ui";
import type { Audit } from "./types";
import "./AuditActivity.css";

type Page = {
  events: Audit[];
  total: number;
  nextCursor?: string;
  actors: { id: string; name: string }[];
};
const categories = [
  "workspace",
  "all",
  "content",
  "sharing",
  "members",
  "settings",
  "security",
  "storage",
  "domain",
] as const;
function categoryLabel(value: string) {
  return (
    (
      {
        workspace: t("워크스페이스 활동"),
        all: t("모든 활동"),
        content: t("콘텐츠"),
        sharing: t("공유"),
        members: t("멤버"),
        settings: t("설정"),
        security: t("보안 및 로그인"),
        storage: t("저장소"),
        domain: t("도메인"),
      } as Record<string, string>
    )[value] || value
  );
}
function actionLabel(action: string) {
  const labels: Record<string, string> = {
    setup: t("워크스페이스 생성"),
    login: t("비밀번호 로그인"),
    "login.google": t("Google 로그인"),
    logout: t("로그아웃"),
    "password.changed": t("비밀번호 변경"),
    "member.created": t("멤버 추가"),
    "member.updated": t("멤버 권한 및 상태 변경"),
    "member.password_reset": t("멤버 비밀번호 초기화"),
    "settings.updated": t("워크스페이스 설정 변경"),
    "profile.updated": t("프로필 변경"),
    "profile.photo_changed": t("프로필 사진 변경"),
    "organization.photo_changed": t("워크스페이스 사진 변경"),
    "auth.google.updated": t("Google 로그인 설정 변경"),
    "token.created": t("API 토큰 생성"),
    "token.revoked": t("API 토큰 해지"),
    "artifact.updated": t("아티팩트 정보 변경"),
    "artifact.sharing": t("공유 대상 변경"),
    "artifact.archive": t("아티팩트 보관"),
    "artifact.unarchive": t("보관 해제"),
    "artifact.restore": t("보관 해제"),
    "artifact.rollback": t("이전 버전으로 변경"),
    "artifact.version_created": t("새 버전 업로드"),
    "artifact.quota_evicted": t("용량 정책에 따라 제거"),
    "artifact.trashed": t("휴지통으로 이동"),
    "artifact.restored": t("휴지통에서 복원"),
    "artifact.purged": t("아티팩트 영구 삭제"),
    "artifact.trash_expired": t("휴지통 보관 기간 만료"),
    "artifact.favorite": t("즐겨찾기 변경"),
    "artifact.context_updated": t("태그 및 생성 정보 변경"),
    "artifact.reviewers": t("리뷰어 변경"),
    "artifact.promoted": t("검토 버전 게시"),
    "review.updated": t("검토 상태 변경"),
    "comment.created": t("댓글 작성"),
    "comment.updated": t("댓글 상태 변경"),
    "comment.deleted": t("댓글 삭제"),
    "collection.saved": t("컬렉션 저장"),
    "collection.deleted": t("컬렉션 삭제"),
    "guest_link.created": t("게스트 공유 링크 생성"),
    "guest_link.revoked": t("게스트 공유 링크 해지"),
    "storage.created": t("저장소 추가"),
    "storage.deleted": t("저장소 삭제"),
    "storage.default_changed": t("기본 저장소 변경"),
    "storage.tested": t("저장소 연결 확인"),
    "storage.test_failed": t("저장소 연결 실패"),
    "storage.gc_failed": t("저장소 정리 실패"),
    "storage.gc_recovered": t("저장소 정리 재개"),
    "storage.close_failed": t("저장소 연결 종료 실패"),
    "cleanup.executed": t("정리 정책 실행"),
    "backup.created": t("백업 생성"),
    "backup.restored": t("백업 복원"),
    "domain.created": t("도메인 추가"),
    "domain.deleted": t("도메인 삭제"),
    "domain.verified": t("도메인 연결 확인"),
    "domain.authorized": t("도메인 권한 확인"),
    "domain.browser_dns_prepared": t("브라우저 DNS 연결 준비"),
    "domain.ownership_provisioned": t("도메인 소유권 레코드 생성"),
    "domain.provisioned": t("도메인 연결 설정"),
    "domain.provision_failed": t("도메인 연결 실패"),
    "domain.routing_provisioned": t("도메인 라우팅 생성"),
    "domain.routing_deleted": t("도메인 라우팅 삭제"),
    "domain.ownership_deleted": t("도메인 소유권 레코드 삭제"),
    "domain.manual_dns_cleanup_required": t("DNS 수동 정리 필요"),
    "dns.cloudflare_connected": t("Cloudflare 연결"),
    "dns.login_started": t("DNS 서비스 로그인 시작"),
    "dns.tunnel_changed": t("터널 설정 변경"),
  };
  return labels[action] || t("기타 활동");
}
function fieldLabel(field: string) {
  return (
    (
      {
        title: t("제목"),
        name: t("이름"),
        visibility: t("공개 범위"),
        members: t("공유 멤버"),
        archived: t("보관 여부"),
        reviewEnabled: t("검토 사용"),
        reviewers: t("리뷰어"),
        currentVersion: t("현재 버전"),
        currentVersionId: t("현재 버전"),
        publishedVersion: t("게시 버전"),
        publishedVersionId: t("게시 버전"),
        tags: t("태그"),
        deleted: t("삭제 여부"),
        reviewStatus: t("검토 상태"),
        favorite: t("즐겨찾기"),
        resolved: t("해결 여부"),
        role: t("역할"),
        disabled: t("계정 비활성화"),
        passwordChangeRequired: t("비밀번호 변경 필요"),
        photoConfigured: t("사진 등록 여부"),
        artifacts: t("포함된 아티팩트"),
        artifactCount: t("아티팩트 수"),
        provider: t("서비스"),
        isDefault: t("기본 저장소"),
        hostname: t("호스트 이름"),
        status: t("연결 상태"),
        artifactId: t("아티팩트"),
        versionId: t("버전"),
        expiresAt: t("만료 시각"),
        revoked: t("해지 여부"),
        passwordProtected: t("비밀번호 보호"),
        scopes: t("토큰 권한"),
        quotaBytes: t("총 용량 한도"),
        maxUploadBytes: t("업로드 한도"),
        retentionDays: t("보관 일수"),
        overflowPolicy: t("용량 초과 정책"),
        googleEnabled: t("Google 로그인 사용"),
        allowedDomains: t("허용 도메인"),
      } as Record<string, string>
    )[field] || t("변경 항목")
  );
}
function valueLabel(value: unknown, field: string): string {
  if (value === null || value === undefined || value === "") return t("없음");
  if (typeof value === "boolean") return value ? t("예") : t("아니요");
  if (Array.isArray(value))
    return value.length
      ? value.map((v) => valueLabel(v, field)).join(", ")
      : t("없음");
  if (typeof value === "number" && field.endsWith("Bytes")) return bytes(value);
  if (
    typeof value === "number" &&
    ["currentVersion", "publishedVersion"].includes(field)
  )
    return `v${value}`;
  if (field === "expiresAt") return date(String(value));
  const known: Record<string, string> = {
    private: t("나만 보기"),
    organization: t("조직 공개"),
    selected: t("선택한 멤버"),
    public: t("링크 공개"),
    admin: t("관리자"),
    member: t("멤버"),
    reject: t("업로드 거부"),
    delete_oldest: t("오래된 항목 삭제"),
    draft: t("초안"),
    in_review: t("검토 중"),
    approved: t("승인됨"),
    changes_requested: t("수정 요청"),
    read: t("읽기"),
    publish: t("게시"),
    review: t("검토"),
  };
  return known[String(value)] || String(value);
}
// Lead with the consequence of the action; keep every other recorded field
// available in expanded details and CSV without altering persisted history.
function prioritizedChanges(event: Audit) {
  const priorities: Record<string, string[]> = {
    "artifact.version_created": [
      "currentVersion",
      "currentVersionId",
      "title",
      "publishedVersion",
      "publishedVersionId",
      "tags",
      "visibility",
    ],
    "artifact.rollback": [
      "currentVersion",
      "currentVersionId",
      "title",
      "tags",
    ],
    "artifact.sharing": ["visibility", "members", "expiresAt"],
    "artifact.reviewers": ["reviewers", "reviewEnabled"],
    "artifact.promoted": [
      "publishedVersion",
      "publishedVersionId",
      "reviewStatus",
    ],
    "review.updated": [
      "reviewStatus",
      "publishedVersion",
      "publishedVersionId",
    ],
    "artifact.updated": ["title", "tags", "visibility"],
    "artifact.context_updated": ["tags", "title"],
    "artifact.trashed": ["deleted"],
    "artifact.restored": ["deleted"],
    "artifact.archive": ["archived"],
    "artifact.restore": ["archived"],
    "member.updated": ["role", "disabled", "name", "passwordChangeRequired"],
    "settings.updated": [
      "name",
      "quotaBytes",
      "maxUploadBytes",
      "retentionDays",
      "overflowPolicy",
    ],
    "collection.saved": ["artifacts", "name", "visibility"],
    "guest_link.created": ["versionId", "expiresAt", "passwordProtected"],
    "guest_link.revoked": ["revoked", "expiresAt"],
  };
  const fields = priorities[event.action] || [
    "title",
    "name",
    "visibility",
    "role",
    "disabled",
    "currentVersion",
    "publishedVersion",
  ];
  const rank = (field: string) => {
    const index = fields.indexOf(field);
    return index < 0 ? fields.length : index;
  };
  return [...(event.changes || [])].sort(
    (a, b) => rank(a.field) - rank(b.field),
  );
}

function Change({ change }: { change: NonNullable<Audit["changes"]>[number] }) {
  return (
    <span className="audit-change">
      <span>{fieldLabel(change.field)}</span>
      <del>{valueLabel(change.before, change.field)}</del>
      <span aria-hidden="true">→</span>
      <ins>{valueLabel(change.after, change.field)}</ins>
    </span>
  );
}
function localBoundary(value: string, nextDay = false) {
  const d = new Date(`${value}T00:00:00`);
  if (nextDay) d.setDate(d.getDate() + 1);
  return d.toISOString();
}

export function AuditActivity() {
  useLocale();
  const [category, setCategory] = useState("workspace");
  const [actor, setActor] = useState("");
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [cursor, setCursor] = useState("");
  const [page, setPage] = useState<Page | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const [revision, setRevision] = useState(0);
  const [expanded, setExpanded] = useState<string | null>(null);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      setQuery(search);
      setCursor("");
    }, 250);
    return () => window.clearTimeout(timer);
  }, [search]);
  const path = useMemo(() => {
    const params = new URLSearchParams({ category, limit: "50" });
    if (actor) params.set("actor", actor);
    if (query) params.set("q", query);
    if (from) params.set("from", localBoundary(from));
    if (to) params.set("to", localBoundary(to, true));
    if (cursor) params.set("cursor", cursor);
    return `/api/admin/audit?${params}`;
  }, [category, actor, query, from, to, cursor]);
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    api<Page>(path)
      .then((data) => {
        if (active)
          setPage((old) =>
            cursor && old
              ? { ...data, events: [...old.events, ...data.events] }
              : data,
          );
      })
      .catch((e) => {
        if (active) setError(e);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [path, revision]);
  function filter(update: () => void) {
    update();
    setCursor("");
    setExpanded(null);
  }
  function exportCSV() {
    // This exports exactly the loaded rows, not a misleading partial "all" export.
    const cell = (value: string) =>
      `"${(/^[\s]*[=+@\-\t\r]/.test(value) ? "'" + value : value).replace(/"/g, '""')}"`;
    const rows = [
      [t("시각"), t("작업자"), t("활동"), t("대상"), t("변경 내용")],
      ...(page?.events || []).map((a) => [
        date(a.createdAt),
        a.actorName || t("시스템"),
        actionLabel(a.action),
        a.targetName || a.targetId,
        prioritizedChanges(a)
          .map(
            (c) =>
              `${fieldLabel(c.field)}: ${valueLabel(c.before, c.field)} → ${valueLabel(c.after, c.field)}`,
          )
          .join("; "),
      ]),
    ];
    const url = URL.createObjectURL(
      new Blob(
        ["\uFEFF" + rows.map((row) => row.map(cell).join(",")).join("\r\n")],
        { type: "text/csv;charset=utf-8" },
      ),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "folio-activity.csv";
    link.click();
    URL.revokeObjectURL(url);
  }
  return (
    <section className="audit-activity" aria-labelledby="audit-title">
      <header className="audit-heading">
        <div>
          <h1 id="audit-title">{t("활동 기록")}</h1>
          <p>
            {t(
              "누가 무엇을 변경했는지 확인하세요. 로그인 기록은 보안 필터에서 볼 수 있습니다.",
            )}
          </p>
        </div>
        <div className="audit-heading-actions">
          <button
            type="button"
            onClick={() => {
              setCursor("");
              setRevision((v) => v + 1);
            }}
            disabled={loading}
            aria-label={t("새로고침")}
          >
            <RefreshCw size={14} />
          </button>
          <button
            type="button"
            onClick={exportCSV}
            disabled={!page?.events.length || loading}
          >
            <Download size={14} />
            {t("표시된 기록 내보내기")}
          </button>
        </div>
      </header>
      <div className="audit-toolbar">
        <Select
          aria-label={t("활동 종류")}
          value={category}
          onValueChange={(v) => filter(() => setCategory(v))}
        >
          {categories.map((v) => (
            <option key={v} value={v}>
              {categoryLabel(v)}
            </option>
          ))}
        </Select>
        <Select
          aria-label={t("작업자")}
          value={actor}
          onValueChange={(v) => filter(() => setActor(v))}
        >
          <option value="">{t("모든 작업자")}</option>
          {page?.actors.map((a) => (
            <option key={a.id || "__system__"} value={a.id || "__system__"}>
              {a.name || (a.id ? a.id : t("시스템"))}
            </option>
          ))}
        </Select>
        <label className="audit-search">
          <Search size={14} />
          <input
            aria-label={t("활동 검색")}
            placeholder={t("작업자, 대상, 변경 내용 검색")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </label>
        <label className="audit-date">
          <span>{t("시작일")}</span>
          <input
            type="date"
            aria-label={t("시작일")}
            value={from}
            max={to || undefined}
            onChange={(e) => filter(() => setFrom(e.target.value))}
          />
        </label>
        <label className="audit-date">
          <span>{t("종료일")}</span>
          <input
            type="date"
            aria-label={t("종료일")}
            value={to}
            min={from || undefined}
            onChange={(e) => filter(() => setTo(e.target.value))}
          />
        </label>
      </div>
      <div className="audit-summary" role="status">
        {loading
          ? t("활동을 불러오는 중…")
          : page
            ? t("전체 {total}건 중 {shown}건 표시", {
                total: page.total,
                shown: page.events.length,
              })
            : ""}
        {category === "security" && (
          <span>{t("비밀번호와 토큰 값은 기록하지 않습니다.")}</span>
        )}
      </div>
      {error ? (
        <ErrorBox error={error} />
      ) : loading && !page ? (
        <Spinner />
      ) : (
        <div className="audit-table-wrap">
          <table className="audit-table">
            <thead>
              <tr>
                <th>{t("시각")}</th>
                <th>{t("작업자")}</th>
                <th>{t("활동 및 대상")}</th>
                <th>{t("변경 내용")}</th>
              </tr>
            </thead>
            <tbody>
              {page?.events
                .map((event) => ({
                  ...event,
                  changes: prioritizedChanges(event),
                }))
                .map((a) => (
                  <tr key={a.id}>
                    <td className="audit-time">
                      <time dateTime={a.createdAt}>{date(a.createdAt)}</time>
                    </td>
                    <td
                      className="audit-actor"
                      title={a.actorName || t("시스템")}
                    >
                      {a.actorName || t("시스템")}
                    </td>
                    <td>
                      <div className="audit-action">
                        {actionLabel(a.action)}
                      </div>
                      <div className="audit-target">
                        {a.targetUrl ? (
                          <a href={a.targetUrl}>
                            {a.targetName || a.targetId || t("워크스페이스")}
                          </a>
                        ) : (
                          <span>
                            {a.targetName || a.targetId || t("워크스페이스")}
                          </span>
                        )}
                        {a.targetNameCurrent && (
                          <span
                            className="audit-current"
                            title={t(
                              "과거 이름이 기록되지 않아 현재 이름을 표시합니다.",
                            )}
                          >
                            {t("현재 이름")}
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="audit-diff">
                      {a.changes?.length ? (
                        <>
                          <Change change={a.changes[0]} />
                          {a.changes.length > 1 && (
                            <button
                              type="button"
                              className="audit-expand"
                              aria-expanded={expanded === a.id}
                              onClick={() =>
                                setExpanded(expanded === a.id ? null : a.id)
                              }
                            >
                              {t("변경 {count}개 더 보기", {
                                count: a.changes.length - 1,
                              })}
                              <ChevronDown size={12} />
                            </button>
                          )}
                          {expanded === a.id && (
                            <div className="audit-more">
                              {a.changes.slice(1).map((c) => (
                                <Change key={c.field} change={c} />
                              ))}
                            </div>
                          )}
                        </>
                      ) : (
                        <span className="audit-muted">
                          {a.snapshot
                            ? t("상세 값 변경 기록 없음")
                            : t(
                                "이전 기록: 변경 전후 값이 저장되지 않았습니다.",
                              )}
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
          {!page?.events.length && (
            <div className="audit-empty">
              {t("조건에 맞는 활동이 없습니다.")}
            </div>
          )}
        </div>
      )}
      {page?.nextCursor && !error && (
        <div className="audit-pagination">
          <button
            type="button"
            disabled={loading}
            onClick={() => setCursor(page.nextCursor || "")}
          >
            {loading ? t("불러오는 중…") : t("이전 활동 더 보기")}
          </button>
        </div>
      )}
      <p className="audit-footnote">
        {t(
          "변경 값은 기록 기능이 추가된 이후의 활동에만 표시됩니다. 콘텐츠 본문과 비밀 값은 저장하지 않습니다.",
        )}
      </p>
    </section>
  );
}
