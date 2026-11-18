import { AuditActivity } from "./AuditActivity";
import { AgentClientGuides } from "./AgentClientGuides";
import {
  ServiceIcon,
  resolveStorageService,
  type ServiceId,
} from "./ServiceIcon";
import {
  UserProfileSettings,
  OrganizationIdentitySettings,
} from "./ProfilePhotos";
import { Select } from "./Select";
import { t as translate, useLocale } from "./i18n";
import { useEffect, useState } from "react";
import {
  Archive,
  ArrowUpRight,
  CheckCircle2,
  CloudUpload,
  ExternalLink,
  Globe,
  HardDrive,
  KeyRound,
  Link,
  Plus,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { api, bytes, date, json, setCSRF } from "./api";
import { GoogleAuthSettings } from "./GoogleAuthSettings";
import "./ResponsiveAdmin.css";
import "./AdminSurface.css";
import type {
  Artifact,
  Backend,
  Domain,
  Organization,
  Policy,
  Session,
  Token,
  User,
} from "./types";
import {
  Avatar,
  CopyButton,
  ErrorBox,
  Field,
  Modal,
  Notice,
  Spinner,
  useAction,
} from "./ui";

export function useResource<T>(path: string, revision = 0) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [key, setKey] = useState(0);
  const requestKey = JSON.stringify([path, revision, key]);
  const [resolvedKey, setResolvedKey] = useState("");
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    api<T>(path)
      .then((r) => {
        if (active) setData(r);
      })
      .catch((e) => {
        if (active) {
          setError(e);
          setData(null);
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
          setResolvedKey(requestKey);
        }
      });
    return () => {
      active = false;
    };
  }, [path, revision, key]);
  return {
    data,
    error,
    loading,
    stale: requestKey !== resolvedKey,
    reload: () => setKey((n) => n + 1),
    setData,
  };
}
export function PageHeading({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  useLocale();
  return (
    <header className="page-heading">
      <div>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {action}
    </header>
  );
}

export function MembersPage({ self }: { self: User }) {
  useLocale();
  const r = useResource<{ members: User[] }>("/api/members");
  const providers = useResource<{ google: { enabled: boolean } }>(
    "/api/auth/providers",
  );
  const [show, setShow] = useState(false);
  const [temporary, setTemporary] = useState("");
  const [created, setCreated] = useState("");
  const [form, setForm] = useState({
    name: "",
    email: "",
    role: "member",
    loginMethod: "password",
  });
  const [confirm, setConfirm] = useState<User | null>(null);
  const a = useAction();
  const members = r.data?.members || [];
  return (
    <div className="admin-surface admin-members">
      <PageHeading
        title={translate("구성원")}
        description={translate(
          "구성원 계정을 만들고 조직의 접근 권한을 관리합니다.",
        )}
        action={
          <button className="button primary" onClick={() => setShow(true)}>
            <Plus size={17} />
            {translate("구성원 추가")}
          </button>
        }
      />
      <ErrorBox error={r.error || a.error} />
      {created && <Notice>{created}</Notice>}
      {temporary && (
        <div className="secret-box">
          <KeyRound size={21} />
          <div>
            <b>{translate("임시 비밀번호가 생성되었습니다.")}</b>
            <p>
              {translate(
                "이 비밀번호는 다시 표시하지 않습니다. 구성원은 첫 로그인 시 비밀번호를 변경합니다.",
              )}
            </p>
            <code>{temporary}</code>
          </div>
          <CopyButton text={temporary} />
          <button
            className="icon-button"
            aria-label={translate("비밀번호 표시 닫기")}
            onClick={() => setTemporary("")}
          >
            ×
          </button>
        </div>
      )}
      <section className="panel members-panel">
        <div className="section-title">
          <span className="admin-result-count">
            {translate("{p0}명", { p0: members.length })}
          </span>
          <span className="subtle">
            {translate("초대 메일 없이 계정을 직접 생성합니다.")}
          </span>
        </div>
        {r.loading ? (
          <div className="loading-row">
            <Spinner />
            {translate("구성원 불러오는 중")}
          </div>
        ) : (
          <div className="table-scroll">
            <table
              className="members-table"
              aria-label={translate("조직 구성원")}
            >
              <thead>
                <tr>
                  <th scope="col">{translate("구성원")}</th>
                  <th scope="col">{translate("역할")}</th>
                  <th scope="col">{translate("상태")}</th>
                  <th scope="col">{translate("가입일")}</th>
                  <th scope="col">
                    <span className="sr-only">{translate("관리")}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {members.map((u) => (
                  <tr key={u.id}>
                    <td>
                      <div className="person-cell">
                        <Avatar name={u.name} src={u.avatarUrl} />
                        <span>
                          <b>
                            {u.name}
                            {u.id === self.id && (
                              <small className="you-label">
                                {translate("나")}
                              </small>
                            )}
                          </b>
                          <small>{u.email}</small>
                        </span>
                      </div>
                    </td>
                    <td data-label={translate("역할")}>
                      <Select
                        aria-label={translate("{p0} 역할", { p0: u.name })}
                        value={u.role}
                        disabled={a.busy || u.id === self.id}
                        onValueChange={(value) =>
                          void a.run(async () => {
                            await api(
                              `/api/admin/members/${u.id}`,
                              json("PATCH", { role: value }),
                            );
                            r.reload();
                          })
                        }
                      >
                        <option value="member">{translate("구성원")}</option>
                        <option value="admin">{translate("관리자")}</option>
                      </Select>
                    </td>
                    <td data-label={translate("상태")}>
                      <span className={`badge ${u.disabled ? "muted" : ""}`}>
                        {u.disabled
                          ? translate("일시 중지")
                          : u.mustChangePassword
                            ? translate("첫 로그인 대기")
                            : translate("활성")}
                      </span>
                    </td>
                    <td className="subtle" data-label={translate("가입일")}>
                      {date(u.createdAt)}
                    </td>
                    <td>
                      <div className="button-row">
                        {u.loginMethod === "google" ? (
                          <span
                            className="subtle"
                            title={translate(
                              "Google에서 비밀번호를 관리하는 계정입니다.",
                            )}
                          >
                            {translate("Google 로그인")}
                          </span>
                        ) : (
                          <button
                            className="button secondary compact"
                            disabled={a.busy || u.id === self.id}
                            title={
                              u.id === self.id
                                ? translate("내 계정에서 비밀번호를 변경하세요")
                                : undefined
                            }
                            onClick={() => setConfirm(u)}
                          >
                            <KeyRound size={14} />
                            {translate("초기화")}
                          </button>
                        )}
                        {u.id !== self.id && (
                          <button
                            className="button secondary compact"
                            disabled={a.busy}
                            onClick={() =>
                              void a.run(async () => {
                                await api(
                                  `/api/admin/members/${u.id}`,
                                  json("PATCH", { disabled: !u.disabled }),
                                );
                                r.reload();
                              })
                            }
                          >
                            {u.disabled
                              ? translate("다시 활성화")
                              : translate("중지")}
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      {show && (
        <Modal
          title={translate("구성원 계정 만들기")}
          onClose={() => setShow(false)}
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                const result = await api<{ temporaryPassword?: string }>(
                  "/api/admin/members",
                  json("POST", form),
                );
                setTemporary(result.temporaryPassword || "");
                setCreated(
                  form.loginMethod === "google"
                    ? translate(
                        "{p0}님의 Google 로그인 계정을 만들었습니다. 등록한 이메일의 Google 계정으로 로그인할 수 있습니다.",
                        { p0: form.name },
                      )
                    : "",
                );
                setShow(false);
                setForm({
                  name: "",
                  email: "",
                  role: "member",
                  loginMethod: "password",
                });
                r.reload();
              });
            }}
          >
            <div className="modal-body">
              <Field label={translate("이름")}>
                <input
                  required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </Field>
              <Field label={translate("이메일")}>
                <input
                  required
                  type="email"
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                />
              </Field>
              <Field label={translate("역할")}>
                <Select
                  value={form.role}
                  onValueChange={(value) => setForm({ ...form, role: value })}
                >
                  <option value="member">{translate("구성원")}</option>
                  <option value="admin">{translate("관리자")}</option>
                </Select>
              </Field>
              {providers.data?.google.enabled && (
                <Field label={translate("로그인 방법")}>
                  <Select
                    value={form.loginMethod}
                    onValueChange={(value) =>
                      setForm({ ...form, loginMethod: value })
                    }
                  >
                    <option value="password">
                      {translate("이메일과 비밀번호")}
                    </option>
                    <option value="google" data-service="google">
                      {translate("Google 계정")}
                    </option>
                  </Select>
                </Field>
              )}
              <Notice>
                {form.loginMethod === "google"
                  ? translate(
                      "등록한 이메일과 일치하는 Google 계정으로만 로그인합니다. 임시 비밀번호는 생성하지 않습니다. 허용 도메인을 설정했다면 해당 Google Workspace 계정이어야 합니다.",
                    )
                  : translate(
                      "임시 비밀번호를 생성합니다. 계정 생성 후 안전한 방법으로 전달해 주세요.",
                    )}
              </Notice>
              {providers.loading && (
                <p className="subtle">
                  {translate("Google 로그인 사용 여부를 확인하고 있습니다.")}
                </p>
              )}
              {!!providers.error && (
                <p className="subtle">
                  {translate(
                    "Google 로그인 설정을 불러오지 못했습니다. 비밀번호 계정을 만들 수 있습니다.",
                  )}
                </p>
              )}
              <ErrorBox error={a.error} />
            </div>
            <footer className="dialog-footer">
              <button
                type="button"
                className="button secondary"
                onClick={() => setShow(false)}
              >
                {translate("취소")}
              </button>
              <button className="button primary" disabled={a.busy}>
                {a.busy ? <Spinner /> : translate("계정 만들기")}
              </button>
            </footer>
          </form>
        </Modal>
      )}
      {confirm && (
        <Modal
          title={translate("비밀번호 초기화")}
          onClose={() => setConfirm(null)}
        >
          <div className="modal-body">
            <p>
              {translate(
                "{p0}님의 비밀번호를 초기화할까요? 기존 로그인과 API 토큰이 해제됩니다.",
                { p0: confirm.name },
              )}
            </p>
            <ErrorBox error={a.error} />
          </div>
          <footer className="dialog-footer">
            <button
              className="button secondary"
              onClick={() => setConfirm(null)}
            >
              {translate("취소")}
            </button>
            <button
              className="button primary"
              disabled={a.busy}
              onClick={() =>
                void a.run(async () => {
                  const result = await api<{ temporaryPassword: string }>(
                    `/api/admin/members/${confirm.id}/reset-password`,
                    json("POST"),
                  );
                  setTemporary(result.temporaryPassword);
                  setConfirm(null);
                  r.reload();
                })
              }
            >
              {translate("초기화")}
            </button>
          </footer>
        </Modal>
      )}
    </div>
  );
}

const getPresets = () => [
  {
    id: "local",
    name: translate("로컬 저장소"),
    provider: "local",
    description: translate("서버의 디스크에 보관"),
  },
  {
    id: "aws",
    name: "Amazon S3",
    provider: "s3",
    description: translate("AWS S3 버킷"),
  },
  {
    id: "r2",
    name: "Cloudflare R2",
    provider: "s3",
    description: translate("S3 호환 객체 저장소"),
  },
  {
    id: "minio",
    name: translate("MinIO / S3 호환"),
    provider: "s3",
    description: translate("사내 또는 호환 저장소"),
  },
  {
    id: "gcs",
    name: "Google Cloud Storage",
    provider: "gcs",
    description: translate("Google Cloud 버킷"),
  },
  {
    id: "azure",
    name: "Azure Blob Storage",
    provider: "azure",
    description: translate("Azure 스토리지 컨테이너"),
  },
];
const defaultBackend = {
  name: "",
  provider: "s3",
  bucket: "",
  region: "",
  endpoint: "",
  prefix: "folio",
  accountName: "",
  accessKeyId: "",
  secretAccessKey: "",
  sessionToken: "",
  serviceAccountJson: "",
  connectionString: "",
  accountKey: "",
};
export function StoragePage() {
  useLocale();
  const r = useResource<{ backends: Backend[] }>("/api/storage");
  const settings = useResource<{ policy: Policy }>("/api/admin/settings");
  const usage = useResource<{
    usage: {
      usedBytes: number;
      quotaBytes: number;
      pendingDeletionBytes?: number;
    };
  }>("/api/artifacts?view=all");
  const [show, setShow] = useState(false);
  const [preset, setPreset] = useState("aws");
  const [form, setForm] = useState(defaultBackend);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [feedback, setFeedback] = useState("");
  const [cleanup, setCleanup] = useState<{
    candidates: {
      id: string;
      title: string;
      reason: string;
      sizeBytes: number;
    }[];
    freedBytes: number;
  } | null>(null);
  const a = useAction();
  useEffect(() => {
    if (settings.data) setPolicy(settings.data.policy);
  }, [settings.data]);
  function select(id: string) {
    setPreset(id);
    const p = getPresets().find((x) => x.id === id)!;
    setForm({
      ...defaultBackend,
      name: p.name,
      provider: p.provider,
      region: id === "r2" ? "auto" : "",
    });
  }
  const p = policy;
  return (
    <div className="admin-surface admin-storage">
      <PageHeading
        title={translate("저장소와 보관")}
        description={translate(
          "데이터는 선택한 저장소에, 보관 정책은 팀의 기준에 맞게.",
        )}
        action={
          <button
            className="button primary"
            onClick={() => {
              select("aws");
              setShow(true);
            }}
          >
            <Plus size={17} />
            {translate("저장소 연결")}
          </button>
        }
      />
      <ErrorBox error={r.error || settings.error || usage.error || a.error} />
      {feedback && <Notice>{feedback}</Notice>}
      <section className="panel storage-summary admin-storage-summary">
        <div className="storage-summary-icon">
          <HardDrive size={28} />
        </div>
        <div>
          <span className="storage-caption">{translate("사용 중인 용량")}</span>
          <h2>
            {bytes(usage.data?.usage.usedBytes || 0)}{" "}
            <span>/ {bytes(p?.quotaBytes || 0)}</span>
          </h2>
          <div className="meter">
            <i
              style={{
                width: `${Math.min(100, ((usage.data?.usage.usedBytes || 0) / (p?.quotaBytes || 1)) * 100)}%`,
              }}
            />
          </div>
          <p>
            {translate("아카이브와 이전 버전을 포함한 보관 용량입니다.")}
            {usage.data?.usage.pendingDeletionBytes
              ? translate(" 실제 삭제 대기 {p0}", {
                  p0: bytes(usage.data.usage.pendingDeletionBytes),
                })
              : ""}
          </p>
        </div>
        <span className="badge">
          {p?.overflowPolicy === "delete_oldest"
            ? translate("오래된 항목 자동 정리")
            : translate("한도 초과 시 게시 중지")}
        </span>
      </section>
      <section className="panel">
        <div className="section-title">
          <h2>{translate("연결된 저장소")}</h2>
          <span className="subtle">
            {translate("기본 저장소 변경은 새로 게시하는 버전부터 적용됩니다.")}
          </span>
        </div>
        {r.loading ? (
          <div className="loading-row">
            <Spinner />
          </div>
        ) : (
          r.data?.backends.map((b) => (
            <div className="connection-row" key={b.id}>
              <div className={`provider-icon ${b.provider}`}>
                <ServiceIcon service={resolveStorageService(b)} size={26} />
              </div>
              <div className="connection-name">
                <b>{b.name}</b>
                <small>
                  {b.provider === "local"
                    ? translate("이 서버의 디스크")
                    : `${b.provider.toUpperCase()} · ${b.bucket}`}
                  {b.endpoint ? ` · ${b.endpoint}` : ""}
                </small>
              </div>
              {b.isDefault && (
                <span className="badge">{translate("기본 저장소")}</span>
              )}
              <button
                className="button secondary compact"
                disabled={a.busy}
                onClick={() =>
                  void a.run(async () => {
                    const result = await api<{ ok: boolean; message: string }>(
                      `/api/storage/${b.id}/test`,
                      json("POST"),
                    );
                    if (!result.ok) throw new Error(result.message);
                    setFeedback(
                      translate("{p0}: 쓰기·읽기·삭제 테스트를 통과했습니다.", {
                        p0: b.name,
                      }),
                    );
                  })
                }
              >
                <RefreshCw size={14} />
                {translate("연결 테스트")}
              </button>
              {!b.isDefault && (
                <>
                  <button
                    className="button secondary compact"
                    disabled={a.busy}
                    onClick={() =>
                      void a.run(async () => {
                        await api(`/api/storage/${b.id}/default`, json("POST"));
                        r.reload();
                      })
                    }
                  >
                    {translate("기본으로")}
                  </button>
                  <button
                    className="icon-button danger-text"
                    aria-label={translate("{p0} 연결 제거", { p0: b.name })}
                    disabled={a.busy}
                    onClick={() =>
                      void a.run(async () => {
                        await api(`/api/storage/${b.id}`, json("DELETE"));
                        r.reload();
                      })
                    }
                  >
                    <Trash2 size={16} />
                  </button>
                </>
              )}
            </div>
          ))
        )}
      </section>
      {p && (
        <section className="panel admin-section admin-storage-policy">
          <div className="section-title">
            <h2>{translate("용량과 보관 정책")}</h2>
            <span className="subtle">
              {translate("자동 정리에서 아카이브는 항상 제외합니다.")}
            </span>
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                await api("/api/admin/settings", json("PATCH", { policy: p }));
                settings.reload();
                usage.reload();
                setFeedback(translate("보관 정책을 저장했습니다."));
              });
            }}
          >
            <div className="form-grid">
              <Field label={translate("전체 용량 한도 (MB)")}>
                <input
                  type="number"
                  min={1}
                  required
                  value={Math.round(p.quotaBytes / 1048576)}
                  onChange={(e) =>
                    setPolicy({
                      ...p,
                      quotaBytes: Number(e.target.value) * 1048576,
                    })
                  }
                />
              </Field>
              <Field label={translate("한 번에 올릴 수 있는 용량 (MB)")}>
                <input
                  type="number"
                  min={1}
                  required
                  value={Math.round(p.maxUploadBytes / 1048576)}
                  onChange={(e) =>
                    setPolicy({
                      ...p,
                      maxUploadBytes: Number(e.target.value) * 1048576,
                    })
                  }
                />
              </Field>
              <Field
                label={translate("보관 기간 (일)")}
                hint={translate("0이면 기간에 따른 자동 삭제를 하지 않습니다.")}
              >
                <input
                  type="number"
                  min={0}
                  required
                  value={p.retentionDays}
                  onChange={(e) =>
                    setPolicy({ ...p, retentionDays: Number(e.target.value) })
                  }
                />
              </Field>
              <Field label={translate("용량을 초과하면")}>
                <Select
                  value={p.overflowPolicy}
                  onValueChange={(value) =>
                    setPolicy({
                      ...p,
                      overflowPolicy: value as Policy["overflowPolicy"],
                    })
                  }
                >
                  <option value="reject">
                    {translate("새 게시를 중지하고 알려주기")}
                  </option>
                  <option value="delete_oldest">
                    {translate("오래된 아티팩트부터 자동 삭제")}
                  </option>
                </Select>
              </Field>
            </div>
            {p.overflowPolicy === "delete_oldest" && (
              <div className="warning-note">
                {translate(
                  "새 게시에 필요한 공간을 확보하기 위해 오래된 아티팩트와 그 버전을 삭제합니다. 아카이브는 보존합니다.",
                )}
              </div>
            )}
            <div className="form-footer">
              <button
                className="button secondary"
                type="button"
                disabled={a.busy}
                onClick={() =>
                  void a.run(async () =>
                    setCleanup(
                      await api(
                        "/api/admin/cleanup",
                        json("POST", { dryRun: true }),
                      ),
                    ),
                  )
                }
              >
                <Archive size={16} />
                {translate("정리 대상 미리보기")}
              </button>
              <button className="button primary" disabled={a.busy}>
                {a.busy ? <Spinner /> : translate("정책 저장")}
              </button>
            </div>
          </form>
        </section>
      )}
      {cleanup && (
        <Modal
          title={translate("보관 정책에 따른 정리 대상")}
          wide
          onClose={() => setCleanup(null)}
        >
          <div className="modal-body">
            <p>
              {translate("{p0}개 아티팩트 · 예상 정리 용량 {p1}", {
                p0: cleanup.candidates.length,
                p1: bytes(cleanup.freedBytes),
              })}
            </p>
            {cleanup.candidates.length ? (
              <ul className="cleanup-list">
                {cleanup.candidates.map((c) => (
                  <li key={c.id}>
                    <b>{c.title}</b>
                    <span>
                      {c.reason} · {bytes(c.sizeBytes)}
                    </span>
                  </li>
                ))}
              </ul>
            ) : (
              <Notice>{translate("현재 정리할 대상이 없습니다.")}</Notice>
            )}
            <p className="subtle">
              {translate(
                "삭제 후 복구할 수 없습니다. 아카이브한 자료는 포함되지 않습니다.",
              )}
            </p>
            <ErrorBox error={a.error} />
          </div>
          <footer className="dialog-footer">
            <button
              className="button secondary"
              onClick={() => setCleanup(null)}
            >
              {translate("닫기")}
            </button>
            <button
              className="button danger"
              disabled={!cleanup.candidates.length || a.busy}
              onClick={() =>
                void a.run(async () => {
                  const result = await api<{
                    candidates: unknown[];
                    freedBytes: number;
                  }>("/api/admin/cleanup", json("POST", { dryRun: false }));
                  setCleanup(null);
                  usage.reload();
                  setFeedback(
                    translate("{p0}개 아티팩트 정리를 요청했습니다.", {
                      p0: result.candidates.length,
                    }),
                  );
                })
              }
            >
              {translate("대상 영구 삭제")}
            </button>
          </footer>
        </Modal>
      )}
      {show && (
        <Modal
          title={translate("저장소 연결")}
          wide
          onClose={() => setShow(false)}
        >
          <form
            className="admin-storage-form"
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                await api("/api/storage", json("POST", form));
                setForm(defaultBackend);
                setShow(false);
                r.reload();
                setFeedback(
                  translate(
                    "저장소를 추가했습니다. 연결 테스트 후 기본 저장소로 지정해 주세요.",
                  ),
                );
              });
            }}
          >
            <div className="modal-body">
              <Field label={translate("저장소 종류")}>
                <Select value={preset} onValueChange={(value) => select(value)}>
                  {getPresets().map((x) => (
                    <option
                      value={x.id}
                      key={x.id}
                      data-service={x.id as ServiceId}
                    >
                      {x.name}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label={translate("연결 이름")}>
                <input
                  required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </Field>
              {form.provider !== "local" && (
                <>
                  <Field
                    label={
                      form.provider === "azure"
                        ? translate("컨테이너 이름")
                        : translate("버킷 이름")
                    }
                  >
                    <input
                      required
                      value={form.bucket}
                      onChange={(e) =>
                        setForm({ ...form, bucket: e.target.value })
                      }
                    />
                  </Field>
                  <Field label={translate("객체 경로 접두사")}>
                    <input
                      value={form.prefix}
                      onChange={(e) =>
                        setForm({ ...form, prefix: e.target.value })
                      }
                    />
                  </Field>
                </>
              )}
              {form.provider === "s3" && (
                <>
                  <div className="form-grid">
                    <Field label={translate("리전")}>
                      <input
                        value={form.region}
                        onChange={(e) =>
                          setForm({ ...form, region: e.target.value })
                        }
                        placeholder={
                          preset === "r2" ? "auto" : "ap-northeast-2"
                        }
                      />
                    </Field>
                    <Field
                      label={translate("엔드포인트")}
                      hint={
                        preset === "aws"
                          ? translate(
                              "AWS 기본 엔드포인트를 사용하려면 비워두세요.",
                            )
                          : translate(
                              "저장소 서비스에서 제공한 주소를 입력하세요.",
                            )
                      }
                    >
                      <input
                        type="url"
                        value={form.endpoint}
                        onChange={(e) =>
                          setForm({ ...form, endpoint: e.target.value })
                        }
                        placeholder={
                          preset === "r2"
                            ? "https://ACCOUNT.r2.cloudflarestorage.com"
                            : "https://storage.example.internal"
                        }
                      />
                    </Field>
                  </div>
                  <Field label={translate("액세스 키 ID")}>
                    <input
                      value={form.accessKeyId}
                      onChange={(e) =>
                        setForm({ ...form, accessKeyId: e.target.value })
                      }
                      autoComplete="off"
                    />
                  </Field>
                  <Field label={translate("시크릿 액세스 키")}>
                    <input
                      type="password"
                      value={form.secretAccessKey}
                      onChange={(e) =>
                        setForm({ ...form, secretAccessKey: e.target.value })
                      }
                      autoComplete="new-password"
                    />
                  </Field>
                  <Field label={translate("세션 토큰 (선택)")}>
                    <input
                      type="password"
                      value={form.sessionToken}
                      onChange={(e) =>
                        setForm({ ...form, sessionToken: e.target.value })
                      }
                      autoComplete="off"
                    />
                  </Field>
                  <p className="subtle">
                    {translate(
                      "키를 비우면 서버에 구성된 기본 AWS 자격 증명을 사용합니다.",
                    )}
                  </p>
                </>
              )}
              {form.provider === "gcs" && (
                <>
                  <Field
                    label={translate("서비스 계정 JSON")}
                    hint={translate(
                      "비워두면 서버의 기본 Google 자격 증명을 사용합니다.",
                    )}
                  >
                    <textarea
                      className="code-input"
                      rows={6}
                      value={form.serviceAccountJson}
                      onChange={(e) =>
                        setForm({ ...form, serviceAccountJson: e.target.value })
                      }
                      autoComplete="off"
                      spellCheck={false}
                    />
                  </Field>
                </>
              )}
              {form.provider === "azure" && (
                <>
                  <Field label={translate("연결 문자열 (선택)")}>
                    <input
                      type="password"
                      value={form.connectionString}
                      onChange={(e) =>
                        setForm({ ...form, connectionString: e.target.value })
                      }
                      autoComplete="new-password"
                    />
                  </Field>
                  <div className="form-grid">
                    <Field label={translate("계정 이름")}>
                      <input
                        value={form.accountName}
                        onChange={(e) =>
                          setForm({ ...form, accountName: e.target.value })
                        }
                      />
                    </Field>
                    <Field label={translate("계정 키")}>
                      <input
                        type="password"
                        value={form.accountKey}
                        onChange={(e) =>
                          setForm({ ...form, accountKey: e.target.value })
                        }
                        autoComplete="new-password"
                      />
                    </Field>
                  </div>
                  <Field label={translate("서비스 엔드포인트 (선택)")}>
                    <input
                      type="url"
                      value={form.endpoint}
                      onChange={(e) =>
                        setForm({ ...form, endpoint: e.target.value })
                      }
                    />
                  </Field>
                </>
              )}
              <Notice>
                {translate(
                  "자격 증명은 서버에서 암호화해 저장하고 다시 표시하지 않습니다.",
                )}
              </Notice>
              <ErrorBox error={a.error} />
            </div>
            <footer className="dialog-footer">
              <button
                type="button"
                className="button secondary"
                onClick={() => setShow(false)}
              >
                {translate("취소")}
              </button>
              <button className="button primary" disabled={a.busy}>
                {a.busy ? <Spinner /> : translate("저장소 연결")}
              </button>
            </footer>
          </form>
        </Modal>
      )}
    </div>
  );
}

export function DomainsPage() {
  useLocale();
  const r = useResource<{ domains: Domain[] }>("/api/domains");
  const providers = useResource<{
    providers: { id: string; supported: boolean; instructions?: string }[];
  }>("/api/admin/dns/providers");
  const arts = useResource<{ artifacts: Artifact[] }>(
    "/api/artifacts?view=all",
  );
  const connector = useResource<{
    configured: boolean;
    running: boolean;
    healthy: boolean;
    id?: string;
    name?: string;
    hostnames: string[];
  }>("/api/admin/dns/cloudflare/tunnel");
  const [show, setShow] = useState(false);
  const [form, setForm] = useState({
    hostname: "",
    artifactId: "",
    provider: "manual",
    target: "",
  });
  const [token, setToken] = useState("");
  const [feedback, setFeedback] = useState("");
  const [login, setLogin] = useState<{
    id: string;
    authorizationUrl: string;
    status: string;
  } | null>(null);
  const [tunnelName, setTunnelName] = useState("folio");
  const [hostnames, setHostnames] = useState("");
  const a = useAction();
  useEffect(() => {
    if (!login || !["pending", "waiting", "running"].includes(login.status))
      return;
    const timer = setInterval(() => {
      api<{ status: string; error?: string }>(
        `/api/admin/dns/cloudflare/login/${login.id}`,
      )
        .then((s) => {
          setLogin((old) => (old ? { ...old, status: s.status } : null));
          if (s.error) a.setError(new Error(s.error));
          if (
            ["complete", "completed", "success", "connected"].includes(s.status)
          ) {
            setFeedback(translate("Cloudflare 로그인이 완료되었습니다."));
            connector.reload();
          }
        })
        .catch(a.setError);
    }, 2500);
    return () => clearInterval(timer);
  }, [login?.id, login?.status]);
  return (
    <div className="admin-surface admin-domains">
      <PageHeading
        title={translate("도메인")}
        description={translate(
          "내부 도메인 또는 인터넷 주소를 아티팩트에 연결합니다.",
        )}
        action={
          <button className="button primary" onClick={() => setShow(true)}>
            <Plus size={17} />
            {translate("도메인 연결")}
          </button>
        }
      />
      <ErrorBox error={r.error || arts.error || connector.error || a.error} />
      {feedback && <Notice>{feedback}</Notice>}
      <section className="panel">
        <div className="section-title">
          <h2>{translate("연결된 도메인")}</h2>
          <span className="subtle">
            {translate("주소가 공개되어도 아티팩트의 공유 권한은 유지됩니다.")}
          </span>
        </div>
        {r.loading ? (
          <div className="loading-row">
            <Spinner />
          </div>
        ) : !r.data?.domains.length ? (
          <div className="domain-empty">
            <Globe size={26} />
            <div>
              <h3>{translate("아직 연결된 도메인이 없습니다.")}</h3>
              <p>
                {translate("도메인을 추가하면 필요한 DNS 레코드를 안내합니다.")}
              </p>
            </div>
          </div>
        ) : (
          r.data.domains.map((d) => (
            <div className="domain-item" key={d.id}>
              <div className="domain-title">
                <Globe size={20} />
                <div>
                  <b>{d.hostname}</b>
                  <small>
                    {arts.data?.artifacts.find((x) => x.id === d.artifactId)
                      ?.title || translate("아티팩트")}{" "}
                    · {d.provider}
                  </small>
                </div>
                <span
                  className={`badge ${d.status === "verified" ? "" : "muted"}`}
                >
                  {d.status === "verified"
                    ? translate("연결 확인됨")
                    : d.status === "error"
                      ? translate("확인 필요")
                      : translate("DNS 등록 대기")}
                </span>
              </div>
              {d.records.length > 0 && (
                <div className="dns-records">
                  <div className="dns-record header">
                    <span>{translate("유형")}</span>
                    <span>{translate("이름")}</span>
                    <span>{translate("값")}</span>
                    <span />
                  </div>
                  {d.records.map((record, i) => (
                    <div className="dns-record" key={i}>
                      <code>{record.type}</code>
                      <code>{record.name}</code>
                      <code>{record.value}</code>
                      <CopyButton text={record.value} />
                    </div>
                  ))}
                </div>
              )}
              {d.lastError && <p className="inline-error">{d.lastError}</p>}
              <div className="button-row">
                <button
                  className="button secondary compact"
                  disabled={a.busy}
                  onClick={() =>
                    void a.run(async () => {
                      const result = await api<{ domain: Domain }>(
                        `/api/domains/${d.id}/verify`,
                        json("POST"),
                      );
                      r.reload();
                      setFeedback(
                        result.domain.status === "verified"
                          ? translate("{p0} 연결을 확인했습니다.", {
                              p0: d.hostname,
                            })
                          : translate("아직 DNS 등록을 확인하지 못했습니다."),
                      );
                    })
                  }
                >
                  <RefreshCw size={14} />
                  {translate("등록 확인")}
                </button>
                {d.provider === "cloudflare" && (
                  <button
                    className="button secondary compact"
                    disabled={a.busy}
                    onClick={() =>
                      void a.run(async () => {
                        try {
                          await api(
                            `/api/domains/${d.id}/provision`,
                            json("POST"),
                          );
                        } finally {
                          r.reload();
                        }
                        setFeedback(
                          translate(
                            "Cloudflare에 레코드를 등록했습니다. 등록 확인을 실행해 주세요.",
                          ),
                        );
                      })
                    }
                  >
                    <CloudUpload size={14} />
                    {translate("DNS 자동 등록")}
                  </button>
                )}
                {d.provider === "domainconnect" && (
                  <button
                    className="button secondary compact"
                    disabled={a.busy}
                    onClick={() =>
                      void a.run(async () => {
                        const result = await api<{ authorizationUrl: string }>(
                          `/api/domains/${d.id}/authorize`,
                          json("POST"),
                        );
                        setLogin({
                          id: "domainconnect",
                          authorizationUrl: result.authorizationUrl,
                          status: "authorize",
                        });
                      })
                    }
                  >
                    {translate("DNS 서비스에서 승인")}
                  </button>
                )}
                <button
                  className="button secondary compact danger-text"
                  disabled={a.busy}
                  onClick={() =>
                    void a.run(async () => {
                      await api(`/api/domains/${d.id}`, json("DELETE"));
                      r.reload();
                    })
                  }
                >
                  {translate("연결 해제")}
                </button>
              </div>
            </div>
          ))
        )}
      </section>
      <details
        className="admin-disclosure domain-provider"
        open={login ? true : undefined}
      >
        <summary>
          <span className="admin-disclosure-title">
            <ServiceIcon service="cloudflare" size={18} />
            {translate("Cloudflare 연결")}
          </span>
          <span className="subtle">
            {translate("DNS 자동 등록을 사용할 때 연결합니다.")}
          </span>
        </summary>
        <div className="admin-disclosure-body">
          <div className="integration-columns">
            <div>
              <h3>{translate("계정으로 로그인")}</h3>
              <p className="subtle">
                {translate(
                  "공식 cloudflared 로그인으로 계정을 연결합니다. 서버에 cloudflared가 설치되어 있어야 합니다.",
                )}
              </p>
              <button
                className="button secondary"
                disabled={a.busy}
                onClick={() =>
                  void a.run(async () => {
                    const result = await api<{
                      id: string;
                      authorizationUrl: string;
                      status: string;
                      error?: string;
                    }>("/api/admin/dns/cloudflare/login", json("POST"));
                    if (result.error) throw new Error(result.error);
                    if (result.status === "complete")
                      setFeedback(
                        translate("Cloudflare 로그인이 완료되었습니다."),
                      );
                    setLogin(result);
                  })
                }
              >
                <ExternalLink size={16} />
                <ServiceIcon service="cloudflare" />
                {translate("Cloudflare 로그인 시작")}
              </button>
            </div>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void a.run(async () => {
                  const result = await api<{
                    connected: boolean;
                    zones: unknown[];
                  }>(
                    "/api/admin/dns/cloudflare/connect",
                    json("POST", { apiToken: token }),
                  );
                  if (!result.connected)
                    throw new Error(
                      translate("계정 연결을 확인하지 못했습니다."),
                    );
                  setToken("");
                  setFeedback(
                    translate("Cloudflare 연결 완료 · {p0}개 영역 확인", {
                      p0: result.zones.length,
                    }),
                  );
                });
              }}
            >
              <h3>{translate("API 토큰으로 연결")}</h3>
              <Field label={translate("DNS 편집 권한이 있는 API 토큰")}>
                <input
                  required
                  type="password"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  autoComplete="new-password"
                />
              </Field>
              <button className="button secondary" disabled={a.busy || !token}>
                {translate("토큰 연결")}
              </button>
            </form>
          </div>
          {login && (
            <div className="login-progress">
              <span className="badge">{login.status}</span>
              {login.authorizationUrl && (
                <a
                  className="button primary compact"
                  href={login.authorizationUrl}
                  target="_blank"
                  rel="noreferrer"
                >
                  {login.id === "domainconnect"
                    ? translate("DNS 승인 페이지 열기")
                    : translate("Cloudflare 열기")}
                  <ArrowUpRight size={14} />
                </a>
              )}
              <p className="subtle">
                {login.status === "complete"
                  ? translate("Cloudflare 계정 인증을 완료했습니다.")
                  : translate("새 창에서 승인 후 이 페이지로 돌아오세요.")}
              </p>
            </div>
          )}
        </div>
      </details>
      <details className="admin-disclosure domain-tunnel">
        <summary>
          <span className="admin-disclosure-title">
            <Link size={18} />
            {translate("선택 사항 · 인터넷 연결")}
          </span>
          {connector.data && (
            <span className={`badge ${connector.data.healthy ? "" : "muted"}`}>
              {connector.data.healthy
                ? translate("연결 정상")
                : connector.data.running
                  ? translate("연결 확인 중")
                  : translate("연결 안 됨")}
            </span>
          )}
        </summary>
        <div className="admin-disclosure-body">
          <p className="subtle">
            {translate(
              "내부망 서버를 외부에 공개하려면 도메인 등록과 별도로 진입 경로가 필요합니다. 전용 Cloudflare Tunnel을 생성하고 실행할 수 있습니다.",
            )}
          </p>
          <div className="form-grid">
            <Field label={translate("전용 터널 이름")}>
              <input
                value={tunnelName}
                onChange={(e) => setTunnelName(e.target.value)}
              />
            </Field>
            <Field
              label={translate("연결할 호스트 이름")}
              hint={translate("쉼표로 구분한 정확한 호스트 이름을 입력하세요.")}
            >
              <input
                value={hostnames}
                onChange={(e) => setHostnames(e.target.value)}
                placeholder="folio.example.com, demo.example.com"
              />
            </Field>
          </div>
          <div className="button-row">
            <button
              className="button secondary"
              disabled={a.busy || !hostnames.trim()}
              onClick={() =>
                void a.run(async () => {
                  await api(
                    "/api/admin/dns/cloudflare/tunnel",
                    json("POST", {
                      name: tunnelName,
                      hostnames: hostnames
                        .split(",")
                        .map((x) => x.trim())
                        .filter(Boolean),
                      create: true,
                    }),
                  );
                  connector.reload();
                  setFeedback(
                    translate(
                      "전용 터널을 구성했습니다. 실행 버튼으로 연결해 주세요.",
                    ),
                  );
                })
              }
            >
              {translate("전용 터널 구성")}
            </button>
            <button
              className="button primary"
              disabled={a.busy || !connector.data?.configured}
              onClick={() =>
                void a.run(async () => {
                  await api(
                    "/api/admin/dns/cloudflare/tunnel/start",
                    json("POST"),
                  );
                  connector.reload();
                })
              }
            >
              {translate("연결 실행")}
            </button>
            <button
              className="button secondary"
              disabled={a.busy || !connector.data?.running}
              onClick={() =>
                void a.run(async () => {
                  await api(
                    "/api/admin/dns/cloudflare/tunnel/stop",
                    json("POST"),
                  );
                  connector.reload();
                })
              }
            >
              {translate("연결 중지")}
            </button>
            <button
              className="icon-button"
              title={translate("연결 상태 새로고침")}
              aria-label={translate("연결 상태 새로고침")}
              onClick={connector.reload}
            >
              <RefreshCw size={16} />
            </button>
          </div>
          <ErrorBox error={connector.error} />
        </div>
      </details>
      {show && (
        <Modal
          title={translate("아티팩트 도메인 연결")}
          onClose={() => setShow(false)}
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                await api("/api/domains", json("POST", form));
                setShow(false);
                setForm({
                  hostname: "",
                  artifactId: "",
                  provider: "manual",
                  target: "",
                });
                r.reload();
              });
            }}
          >
            <div className="modal-body">
              <Field label={translate("호스트 이름")}>
                <input
                  required
                  value={form.hostname}
                  onChange={(e) =>
                    setForm({ ...form, hostname: e.target.value })
                  }
                  placeholder="demo.example.com"
                  autoCapitalize="none"
                />
              </Field>
              <Field label={translate("연결할 아티팩트")}>
                <Select
                  required
                  value={form.artifactId}
                  onValueChange={(value) =>
                    setForm({ ...form, artifactId: value })
                  }
                >
                  <option value="">{translate("아티팩트 선택")}</option>
                  {arts.data?.artifacts.map((x) => (
                    <option key={x.id} value={x.id}>
                      {x.title}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label={translate("DNS 연결 방식")}>
                <Select
                  value={form.provider}
                  onValueChange={(value) =>
                    setForm({ ...form, provider: value })
                  }
                >
                  <option value="manual">
                    {translate("DNS 레코드 직접 등록")}
                  </option>
                  <option value="cloudflare" data-service="cloudflare">
                    {translate("Cloudflare 자동 등록")}
                  </option>
                  <option
                    data-service="domainconnect"
                    value="domainconnect"
                    disabled={
                      !providers.data?.providers.find(
                        (p) => p.id === "domainconnect",
                      )?.supported
                    }
                  >
                    {translate("Domain Connect · 지원 DNS 서비스")}
                  </option>
                </Select>
              </Field>
              <Field
                label={translate("연결 대상 (선택)")}
                hint={translate(
                  "비워두면 이 아티팩트의 기본 콘텐츠 주소를 사용합니다.",
                )}
              >
                <input
                  value={form.target}
                  onChange={(e) => setForm({ ...form, target: e.target.value })}
                  placeholder="content.example.internal"
                />
              </Field>
              <Notice>
                {translate(
                  "다음 단계에서 소유권 확인용 TXT와 연결 레코드를 안내합니다. 기존 DNS 레코드는 자동으로 덮어쓰지 않습니다.",
                )}
              </Notice>
              <ErrorBox error={a.error} />
            </div>
            <footer className="dialog-footer">
              <button
                type="button"
                className="button secondary"
                onClick={() => setShow(false)}
              >
                {translate("취소")}
              </button>
              <button className="button primary" disabled={a.busy}>
                {a.busy ? <Spinner /> : translate("다음: DNS 등록")}
              </button>
            </footer>
          </form>
        </Modal>
      )}
    </div>
  );
}

export function AgentsPage() {
  useLocale();
  const r = useResource<{ tokens: Token[] }>("/api/tokens");
  const a = useAction();
  return (
    <div className="admin-surface admin-agents">
      <PageHeading
        title={translate("에이전트 연결")}
        description={translate(
          "모델이나 구독에 종속되지 않는 MCP 도구와 CLI를 연결하세요.",
        )}
      />
      <ErrorBox error={r.error || a.error} />
      <AgentClientGuides onConnected={r.reload} />
      <section className="panel">
        <div className="section-title">
          <h2>{translate("연결 관리")}</h2>
          <span className="subtle">
            {translate("폐기하면 연결된 에이전트의 접근도 해제됩니다.")}
          </span>
        </div>
        {!r.data?.tokens.length ? (
          <p className="muted-padded">
            {translate("아직 발급한 토큰이 없습니다.")}
          </p>
        ) : (
          r.data.tokens.map((t) => (
            <div className="connection-row" key={t.id}>
              <KeyRound size={20} />
              <div className="connection-name">
                <b>{t.name}</b>
                <small>
                  {t.scopes
                    .map(
                      (scope) =>
                        ({
                          read: translate("목록·미리보기"),
                          publish: translate("게시·버전 갱신·보관"),
                          share: translate("공유 설정 변경"),
                          review: translate("검토·댓글 작성"),
                        })[scope] || scope,
                    )
                    .join(" · ")}{" "}
                  · {translate("{p0} 발급", { p0: date(t.createdAt) })}
                  {t.expiresAt
                    ? translate(" · {p0} 만료", { p0: date(t.expiresAt) })
                    : ""}
                </small>
              </div>
              <button
                className="button secondary compact danger-text"
                disabled={a.busy}
                onClick={() =>
                  void a.run(async () => {
                    await api(`/api/tokens/${t.id}`, json("DELETE"));
                    r.reload();
                  })
                }
              >
                {translate("폐기")}
              </button>
            </div>
          ))
        )}
      </section>
    </div>
  );
}

export function SettingsPage({ onChanged }: { onChanged: () => void }) {
  useLocale();
  const r = useResource<{
    organization: Organization;
    policy: Policy;
    appUrl: string;
    contentUrl: string;
  }>("/api/admin/settings");
  return (
    <div className="admin-surface admin-settings">
      <PageHeading
        title={translate("조직 설정")}
        description={translate("조직 이름과 설치 정보를 확인합니다.")}
      />
      <ErrorBox error={r.error} />
      {r.data && (
        <OrganizationIdentitySettings
          organization={r.data.organization}
          onSaved={() => {
            r.reload();
            onChanged();
          }}
        />
      )}
      <GoogleAuthSettings />
      <details className="admin-disclosure admin-installation">
        <summary>
          <span className="admin-disclosure-title">
            {translate("설치 정보")}
          </span>
          <span className="subtle">
            {translate("관리 주소와 콘텐츠 호스트")}
          </span>
        </summary>
        <div className="admin-disclosure-body">
          <dl className="detail-list">
            <div>
              <dt>{translate("관리 주소")}</dt>
              <dd>{r.data?.appUrl}</dd>
            </div>
            <div>
              <dt>{translate("아티팩트 주소")}</dt>
              <dd>{r.data?.contentUrl}</dd>
            </div>
            <div>
              <dt>{translate("배포 방식")}</dt>
              <dd>{translate("설치형 · 단일 조직")}</dd>
            </div>
            <div>
              <dt>{translate("라이선스")}</dt>
              <dd>MIT · {translate("오픈 소스")}</dd>
            </div>
          </dl>
          <Notice>
            {translate(
              "아티팩트는 관리 페이지와 분리된 도메인에서 실행합니다. 주소 변경은 서버 설정에서 적용해 주세요.",
            )}
          </Notice>
        </div>
      </details>
    </div>
  );
}
export function AuditPage() {
  return <AuditActivity />;
}

export function AccountPage({
  session,
  onSession,
}: {
  session: Session;
  onSession: (s: Session) => void;
}) {
  useLocale();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [saved, setSaved] = useState(false);
  const a = useAction();
  return (
    <div className="admin-surface admin-account">
      <PageHeading
        title={translate("내 계정")}
        description={
          session.user.loginMethod === "google"
            ? translate("계정 정보와 로그인 방법을 확인합니다.")
            : translate("계정 정보를 확인하고 비밀번호를 변경합니다.")
        }
      />
      <UserProfileSettings
        user={session.user}
        onSaved={(user) => onSession({ ...session, user })}
      />
      {session.user.loginMethod === "google" ? (
        <section className="panel admin-section">
          <div className="admin-section-heading">
            <h2>{translate("로그인 및 보안")}</h2>
          </div>
          <div>
            <p className="subtle">
              {translate(
                "Google 계정으로 로그인하는 계정입니다. 비밀번호는 Google 계정에서 관리하며 Folio 비밀번호는 사용하지 않습니다.",
              )}
            </p>
            <a
              className="button secondary"
              href="https://myaccount.google.com/security"
              target="_blank"
              rel="noreferrer"
            >
              <ServiceIcon service="google" />
              {translate("Google 계정 보안 설정")}
              <ExternalLink size={15} />
            </a>
          </div>
        </section>
      ) : (
        <section className="panel admin-section">
          <div className="admin-section-heading">
            <h2>{translate("비밀번호 변경")}</h2>
            <p>
              {translate(
                "변경하면 다른 로그인 세션과 기존 API 토큰이 해제됩니다.",
              )}
            </p>
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                if (next !== confirm)
                  throw new Error(
                    translate("새 비밀번호가 일치하지 않습니다."),
                  );
                const s = await api<Session>(
                  "/api/auth/password",
                  json("POST", { currentPassword: current, newPassword: next }),
                );
                setCSRF(s.csrfToken);
                onSession(s);
                setCurrent("");
                setNext("");
                setConfirm("");
                setSaved(true);
              });
            }}
          >
            <div className="form-grid">
              <Field label={translate("현재 비밀번호")}>
                <input
                  required
                  type="password"
                  autoComplete="current-password"
                  value={current}
                  onChange={(e) => setCurrent(e.target.value)}
                />
              </Field>
              <div />
            </div>
            <div className="form-grid">
              <Field label={translate("새 비밀번호")}>
                <input
                  required
                  type="password"
                  minLength={12}
                  autoComplete="new-password"
                  value={next}
                  onChange={(e) => setNext(e.target.value)}
                />
              </Field>
              <Field label={translate("새 비밀번호 확인")}>
                <input
                  required
                  type="password"
                  minLength={12}
                  autoComplete="new-password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                />
              </Field>
            </div>
            <ErrorBox error={a.error} />
            <button className="button primary" disabled={a.busy}>
              {a.busy ? <Spinner /> : translate("비밀번호 변경")}
            </button>
            {saved && (
              <span className="saved-inline">
                <CheckCircle2 size={15} />
                {translate("비밀번호를 변경했습니다.")}
              </span>
            )}
          </form>
        </section>
      )}
    </div>
  );
}
