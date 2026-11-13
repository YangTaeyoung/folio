import { useEffect, useState } from "react";
import {
  Archive,
  ArrowUpRight,
  CheckCircle2,
  Cloud,
  CloudUpload,
  ExternalLink,
  Globe,
  HardDrive,
  KeyRound,
  Link,
  Plus,
  RefreshCw,
  Server,
  ShieldCheck,
  Terminal,
  Trash2,
} from "lucide-react";
import { api, bytes, date, json, setCSRF } from "./api";
import type {
  Artifact,
  Audit,
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
  Empty,
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
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    api<T>(path)
      .then((r) => {
        if (active) setData(r);
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
  }, [path, revision, key]);
  return { data, error, loading, reload: () => setKey((n) => n + 1), setData };
}
export function PageHeading({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow: string;
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <header className="page-heading">
      <div>
        <small className="eyebrow">{eyebrow}</small>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {action}
    </header>
  );
}

export function MembersPage({ self }: { self: User }) {
  const r = useResource<{ members: User[] }>("/api/members");
  const [show, setShow] = useState(false);
  const [temporary, setTemporary] = useState("");
  const [form, setForm] = useState({ name: "", email: "", role: "member" });
  const [confirm, setConfirm] = useState<User | null>(null);
  const a = useAction();
  const members = r.data?.members || [];
  return (
    <>
      <PageHeading
        eyebrow="WORKSPACE / MEMBERS"
        title="함께 만드는 사람들"
        description="구성원 계정을 만들고 조직의 접근 권한을 관리합니다."
        action={
          <button className="button primary" onClick={() => setShow(true)}>
            <Plus size={17} />
            구성원 추가
          </button>
        }
      />
      <ErrorBox error={r.error || a.error} />
      {temporary && (
        <div className="secret-box">
          <KeyRound size={21} />
          <div>
            <b>임시 비밀번호가 생성되었습니다.</b>
            <p>
              이 비밀번호는 다시 표시하지 않습니다. 구성원은 첫 로그인 시
              비밀번호를 변경합니다.
            </p>
            <code>{temporary}</code>
          </div>
          <CopyButton text={temporary} />
          <button
            className="icon-button"
            aria-label="비밀번호 표시 닫기"
            onClick={() => setTemporary("")}
          >
            ×
          </button>
        </div>
      )}
      <section className="panel">
        <div className="section-title">
          <h2>
            조직 구성원 <span className="count">{members.length}</span>
          </h2>
          <span className="subtle">초대 메일 없이 계정을 직접 생성합니다.</span>
        </div>
        {r.loading ? (
          <div className="loading-row">
            <Spinner />
            구성원 불러오는 중
          </div>
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>구성원</th>
                  <th>역할</th>
                  <th>상태</th>
                  <th>가입일</th>
                  <th>
                    <span className="sr-only">관리</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {members.map((u) => (
                  <tr key={u.id}>
                    <td>
                      <div className="person-cell">
                        <Avatar name={u.name} />
                        <span>
                          <b>
                            {u.name}
                            {u.id === self.id && (
                              <small className="you-label">나</small>
                            )}
                          </b>
                          <small>{u.email}</small>
                        </span>
                      </div>
                    </td>
                    <td>
                      <select
                        aria-label={`${u.name} 역할`}
                        value={u.role}
                        disabled={a.busy || u.id === self.id}
                        onChange={(e) =>
                          void a.run(async () => {
                            await api(
                              `/api/admin/members/${u.id}`,
                              json("PATCH", { role: e.target.value }),
                            );
                            r.reload();
                          })
                        }
                      >
                        <option value="member">구성원</option>
                        <option value="admin">관리자</option>
                      </select>
                    </td>
                    <td>
                      <span className={`badge ${u.disabled ? "muted" : ""}`}>
                        {u.disabled
                          ? "일시 중지"
                          : u.mustChangePassword
                            ? "첫 로그인 대기"
                            : "활성"}
                      </span>
                    </td>
                    <td className="subtle">{date(u.createdAt)}</td>
                    <td>
                      <div className="button-row">
                        <button
                          className="button secondary compact"
                          disabled={a.busy || u.id === self.id}
                          title={
                            u.id === self.id
                              ? "내 계정에서 비밀번호를 변경하세요"
                              : undefined
                          }
                          onClick={() => setConfirm(u)}
                        >
                          <KeyRound size={14} />
                          초기화
                        </button>
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
                            {u.disabled ? "다시 활성화" : "중지"}
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
        <Modal title="구성원 계정 만들기" onClose={() => setShow(false)}>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                const result = await api<{ temporaryPassword?: string }>(
                  "/api/admin/members",
                  json("POST", form),
                );
                setTemporary(result.temporaryPassword || "");
                setShow(false);
                setForm({ name: "", email: "", role: "member" });
                r.reload();
              });
            }}
          >
            <div className="modal-body">
              <Field label="이름">
                <input
                  required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </Field>
              <Field label="이메일">
                <input
                  required
                  type="email"
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                />
              </Field>
              <Field label="역할">
                <select
                  value={form.role}
                  onChange={(e) => setForm({ ...form, role: e.target.value })}
                >
                  <option value="member">구성원</option>
                  <option value="admin">관리자</option>
                </select>
              </Field>
              <Notice>
                임시 비밀번호를 생성합니다. 계정 생성 후 안전한 방법으로 전달해
                주세요.
              </Notice>
              <ErrorBox error={a.error} />
            </div>
            <footer className="dialog-footer">
              <button
                type="button"
                className="button secondary"
                onClick={() => setShow(false)}
              >
                취소
              </button>
              <button className="button primary" disabled={a.busy}>
                {a.busy ? <Spinner /> : "계정 만들기"}
              </button>
            </footer>
          </form>
        </Modal>
      )}
      {confirm && (
        <Modal title="비밀번호 초기화" onClose={() => setConfirm(null)}>
          <div className="modal-body">
            <p>
              {confirm.name}님의 비밀번호를 초기화할까요? 기존 로그인과 API
              토큰이 해제됩니다.
            </p>
            <ErrorBox error={a.error} />
          </div>
          <footer className="dialog-footer">
            <button
              className="button secondary"
              onClick={() => setConfirm(null)}
            >
              취소
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
              초기화
            </button>
          </footer>
        </Modal>
      )}
    </>
  );
}

const presets = [
  {
    id: "local",
    name: "로컬 저장소",
    provider: "local",
    description: "서버의 디스크에 보관",
  },
  { id: "aws", name: "Amazon S3", provider: "s3", description: "AWS S3 버킷" },
  {
    id: "r2",
    name: "Cloudflare R2",
    provider: "s3",
    description: "S3 호환 객체 저장소",
  },
  {
    id: "minio",
    name: "MinIO / S3 호환",
    provider: "s3",
    description: "사내 또는 호환 저장소",
  },
  {
    id: "gcs",
    name: "Google Cloud Storage",
    provider: "gcs",
    description: "Google Cloud 버킷",
  },
  {
    id: "azure",
    name: "Azure Blob Storage",
    provider: "azure",
    description: "Azure 스토리지 컨테이너",
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
    const p = presets.find((x) => x.id === id)!;
    setForm({
      ...defaultBackend,
      name: p.name,
      provider: p.provider,
      region: id === "r2" ? "auto" : "",
    });
  }
  const p = policy;
  return (
    <>
      <PageHeading
        eyebrow="WORKSPACE / STORAGE"
        title="보관할 곳과 보관하는 방법"
        description="데이터는 선택한 저장소에, 보관 정책은 팀의 기준에 맞게."
        action={
          <button
            className="button primary"
            onClick={() => {
              select("aws");
              setShow(true);
            }}
          >
            <Plus size={17} />
            저장소 연결
          </button>
        }
      />
      <ErrorBox error={r.error || settings.error || usage.error || a.error} />
      {feedback && <Notice>{feedback}</Notice>}
      <section className="panel storage-summary">
        <div className="storage-summary-icon">
          <HardDrive size={28} />
        </div>
        <div>
          <small className="eyebrow">WORKSPACE STORAGE</small>
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
            아카이브와 이전 버전을 포함한 보관 용량입니다.
            {usage.data?.usage.pendingDeletionBytes
              ? ` 실제 삭제 대기 ${bytes(usage.data.usage.pendingDeletionBytes)}`
              : ""}
          </p>
        </div>
        <span className="badge">
          {p?.overflowPolicy === "delete_oldest"
            ? "오래된 항목 자동 정리"
            : "한도 초과 시 게시 중지"}
        </span>
      </section>
      <section className="panel">
        <div className="section-title">
          <h2>연결된 저장소</h2>
          <span className="subtle">
            기본 저장소 변경은 새로 게시하는 버전부터 적용됩니다.
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
                {b.provider === "local" ? (
                  <HardDrive size={23} />
                ) : (
                  <Cloud size={23} />
                )}
              </div>
              <div className="connection-name">
                <b>{b.name}</b>
                <small>
                  {b.provider === "local"
                    ? "이 서버의 디스크"
                    : `${b.provider.toUpperCase()} · ${b.bucket}`}
                  {b.endpoint ? ` · ${b.endpoint}` : ""}
                </small>
              </div>
              {b.isDefault && <span className="badge">기본 저장소</span>}
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
                      `${b.name}: 쓰기·읽기·삭제 테스트를 통과했습니다.`,
                    );
                  })
                }
              >
                <RefreshCw size={14} />
                연결 테스트
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
                    기본으로
                  </button>
                  <button
                    className="icon-button danger-text"
                    aria-label={`${b.name} 연결 제거`}
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
        <section className="panel">
          <div className="section-title">
            <h2>용량과 보관 정책</h2>
            <span className="subtle">
              자동 정리에서 아카이브는 항상 제외합니다.
            </span>
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                await api("/api/admin/settings", json("PATCH", { policy: p }));
                settings.reload();
                usage.reload();
                setFeedback("보관 정책을 저장했습니다.");
              });
            }}
          >
            <div className="form-grid">
              <Field label="전체 용량 한도 (MB)">
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
              <Field label="한 번에 올릴 수 있는 용량 (MB)">
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
                label="보관 기간 (일)"
                hint="0이면 기간에 따른 자동 삭제를 하지 않습니다."
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
              <Field label="용량을 초과하면">
                <select
                  value={p.overflowPolicy}
                  onChange={(e) =>
                    setPolicy({
                      ...p,
                      overflowPolicy: e.target
                        .value as Policy["overflowPolicy"],
                    })
                  }
                >
                  <option value="reject">새 게시를 중지하고 알려주기</option>
                  <option value="delete_oldest">
                    오래된 아티팩트부터 자동 삭제
                  </option>
                </select>
              </Field>
            </div>
            {p.overflowPolicy === "delete_oldest" && (
              <div className="warning-note">
                새 게시에 필요한 공간을 확보하기 위해 오래된 아티팩트와 그
                버전을 삭제합니다. 아카이브는 보존합니다.
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
                정리 대상 미리보기
              </button>
              <button className="button primary" disabled={a.busy}>
                {a.busy ? <Spinner /> : "정책 저장"}
              </button>
            </div>
          </form>
        </section>
      )}
      {cleanup && (
        <Modal
          title="보관 정책에 따른 정리 대상"
          wide
          onClose={() => setCleanup(null)}
        >
          <div className="modal-body">
            <p>
              {cleanup.candidates.length}개 아티팩트 · 예상{" "}
              {bytes(cleanup.freedBytes)} 정리
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
              <Notice>현재 정리할 대상이 없습니다.</Notice>
            )}
            <p className="subtle">
              삭제 후 복구할 수 없습니다. 아카이브한 자료는 포함되지 않습니다.
            </p>
            <ErrorBox error={a.error} />
          </div>
          <footer className="dialog-footer">
            <button
              className="button secondary"
              onClick={() => setCleanup(null)}
            >
              닫기
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
                    `${result.candidates.length}개 아티팩트 정리를 요청했습니다.`,
                  );
                })
              }
            >
              대상 영구 삭제
            </button>
          </footer>
        </Modal>
      )}
      {show && (
        <Modal title="저장소 연결" wide onClose={() => setShow(false)}>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                await api("/api/storage", json("POST", form));
                setForm(defaultBackend);
                setShow(false);
                r.reload();
                setFeedback(
                  "저장소를 추가했습니다. 연결 테스트 후 기본 저장소로 지정해 주세요.",
                );
              });
            }}
          >
            <div className="modal-body">
              <Field label="저장소 종류">
                <select value={preset} onChange={(e) => select(e.target.value)}>
                  {presets.map((x) => (
                    <option value={x.id} key={x.id}>
                      {x.name}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="연결 이름">
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
                      form.provider === "azure" ? "컨테이너 이름" : "버킷 이름"
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
                  <Field label="객체 경로 접두사">
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
                    <Field label="리전">
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
                      label="엔드포인트"
                      hint={
                        preset === "aws"
                          ? "AWS 기본 엔드포인트를 사용하려면 비워두세요."
                          : "저장소 서비스에서 제공한 주소를 입력하세요."
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
                  <Field label="액세스 키 ID">
                    <input
                      value={form.accessKeyId}
                      onChange={(e) =>
                        setForm({ ...form, accessKeyId: e.target.value })
                      }
                      autoComplete="off"
                    />
                  </Field>
                  <Field label="시크릿 액세스 키">
                    <input
                      type="password"
                      value={form.secretAccessKey}
                      onChange={(e) =>
                        setForm({ ...form, secretAccessKey: e.target.value })
                      }
                      autoComplete="new-password"
                    />
                  </Field>
                  <Field label="세션 토큰 (선택)">
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
                    키를 비우면 서버에 구성된 기본 AWS 자격 증명을 사용합니다.
                  </p>
                </>
              )}
              {form.provider === "gcs" && (
                <>
                  <Field
                    label="서비스 계정 JSON"
                    hint="비워두면 서버의 기본 Google 자격 증명을 사용합니다."
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
                  <Field label="연결 문자열 (선택)">
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
                    <Field label="계정 이름">
                      <input
                        value={form.accountName}
                        onChange={(e) =>
                          setForm({ ...form, accountName: e.target.value })
                        }
                      />
                    </Field>
                    <Field label="계정 키">
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
                  <Field label="서비스 엔드포인트 (선택)">
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
                자격 증명은 서버에서 암호화해 저장하고 다시 표시하지 않습니다.
              </Notice>
              <ErrorBox error={a.error} />
            </div>
            <footer className="dialog-footer">
              <button
                type="button"
                className="button secondary"
                onClick={() => setShow(false)}
              >
                취소
              </button>
              <button className="button primary" disabled={a.busy}>
                {a.busy ? <Spinner /> : "저장소 연결"}
              </button>
            </footer>
          </form>
        </Modal>
      )}
    </>
  );
}

export function DomainsPage() {
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
            setFeedback("Cloudflare 로그인이 완료되었습니다.");
            connector.reload();
          }
        })
        .catch(a.setError);
    }, 2500);
    return () => clearInterval(timer);
  }, [login?.id, login?.status]);
  return (
    <>
      <PageHeading
        eyebrow="WORKSPACE / DOMAINS"
        title="작업에 어울리는 주소"
        description="내부 도메인 또는 인터넷 주소를 아티팩트에 연결합니다."
        action={
          <button className="button primary" onClick={() => setShow(true)}>
            <Plus size={17} />
            도메인 연결
          </button>
        }
      />
      <ErrorBox error={r.error || arts.error || a.error} />
      {feedback && <Notice>{feedback}</Notice>}
      <section className="panel">
        <div className="section-title">
          <h2>연결된 도메인</h2>
          <span className="subtle">
            주소가 공개되어도 아티팩트의 공유 권한은 유지됩니다.
          </span>
        </div>
        {r.loading ? (
          <div className="loading-row">
            <Spinner />
          </div>
        ) : !r.data?.domains.length ? (
          <Empty
            icon={<Globe size={30} />}
            heading="내 도메인으로 공유해 보세요"
            description="Cloudflare로 연결하거나, DNS 서비스에 레코드를 직접 등록할 수 있습니다."
          />
        ) : (
          r.data.domains.map((d) => (
            <div className="domain-item" key={d.id}>
              <div className="domain-title">
                <Globe size={20} />
                <div>
                  <b>{d.hostname}</b>
                  <small>
                    {arts.data?.artifacts.find((x) => x.id === d.artifactId)
                      ?.title || "아티팩트"}{" "}
                    · {d.provider}
                  </small>
                </div>
                <span
                  className={`badge ${d.status === "verified" ? "" : "muted"}`}
                >
                  {d.status === "verified"
                    ? "연결 확인됨"
                    : d.status === "error"
                      ? "확인 필요"
                      : "DNS 등록 대기"}
                </span>
              </div>
              {d.records.length > 0 && (
                <div className="dns-records">
                  <div className="dns-record header">
                    <span>유형</span>
                    <span>이름</span>
                    <span>값</span>
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
                          ? `${d.hostname} 연결을 확인했습니다.`
                          : "아직 DNS 등록을 확인하지 못했습니다.",
                      );
                    })
                  }
                >
                  <RefreshCw size={14} />
                  등록 확인
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
                          "Cloudflare에 레코드를 등록했습니다. 등록 확인을 실행해 주세요.",
                        );
                      })
                    }
                  >
                    <CloudUpload size={14} />
                    DNS 자동 등록
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
                    DNS 서비스에서 승인
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
                  연결 해제
                </button>
              </div>
            </div>
          ))
        )}
      </section>
      <section className="panel">
        <div className="section-title">
          <h2>
            <Cloud size={19} />
            Cloudflare 연결
          </h2>
          <span className="subtle">
            외부 연결을 선택한 경우에만 사용합니다.
          </span>
        </div>
        <div className="integration-columns">
          <div>
            <h3>계정으로 로그인</h3>
            <p className="subtle">
              공식 cloudflared 로그인으로 계정을 연결합니다. 서버에
              cloudflared가 설치되어 있어야 합니다.
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
                    setFeedback("Cloudflare 로그인이 완료되었습니다.");
                  setLogin(result);
                })
              }
            >
              <ExternalLink size={16} />
              Cloudflare 로그인 시작
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
                  throw new Error("계정 연결을 확인하지 못했습니다.");
                setToken("");
                setFeedback(
                  `Cloudflare 연결 완료 · ${result.zones.length}개 영역 확인`,
                );
              });
            }}
          >
            <h3>API 토큰으로 연결</h3>
            <Field label="DNS 편집 권한이 있는 API 토큰">
              <input
                required
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                autoComplete="new-password"
              />
            </Field>
            <button className="button secondary" disabled={a.busy || !token}>
              토큰 연결
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
                  ? "DNS 승인 페이지 열기"
                  : "Cloudflare 열기"}
                <ArrowUpRight size={14} />
              </a>
            )}
            <p className="subtle">
              {login.status === "complete"
                ? "Cloudflare 계정 인증을 완료했습니다."
                : "새 창에서 승인 후 이 페이지로 돌아오세요."}
            </p>
          </div>
        )}
      </section>
      <section className="panel">
        <div className="section-title">
          <h2>
            <Link size={19} />
            선택 사항 · 인터넷 연결
          </h2>
          {connector.data && (
            <span className={`badge ${connector.data.healthy ? "" : "muted"}`}>
              {connector.data.healthy
                ? "연결 정상"
                : connector.data.running
                  ? "연결 확인 중"
                  : "연결 안 됨"}
            </span>
          )}
        </div>
        <p className="subtle">
          내부망 서버를 외부에 공개하려면 도메인 등록과 별도로 진입 경로가
          필요합니다. 전용 Cloudflare Tunnel을 생성하고 실행할 수 있습니다.
        </p>
        <div className="form-grid">
          <Field label="전용 터널 이름">
            <input
              value={tunnelName}
              onChange={(e) => setTunnelName(e.target.value)}
            />
          </Field>
          <Field
            label="연결할 호스트 이름"
            hint="쉼표로 구분한 정확한 호스트 이름을 입력하세요."
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
                  "전용 터널을 구성했습니다. 실행 버튼으로 연결해 주세요.",
                );
              })
            }
          >
            전용 터널 구성
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
            연결 실행
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
            연결 중지
          </button>
          <button
            className="icon-button"
            title="연결 상태 새로고침"
            aria-label="연결 상태 새로고침"
            onClick={connector.reload}
          >
            <RefreshCw size={16} />
          </button>
        </div>
        <ErrorBox error={connector.error} />
      </section>
      {show && (
        <Modal title="아티팩트 도메인 연결" onClose={() => setShow(false)}>
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
              <Field label="호스트 이름">
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
              <Field label="연결할 아티팩트">
                <select
                  required
                  value={form.artifactId}
                  onChange={(e) =>
                    setForm({ ...form, artifactId: e.target.value })
                  }
                >
                  <option value="">아티팩트 선택</option>
                  {arts.data?.artifacts.map((x) => (
                    <option key={x.id} value={x.id}>
                      {x.title}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="DNS 연결 방식">
                <select
                  value={form.provider}
                  onChange={(e) =>
                    setForm({ ...form, provider: e.target.value })
                  }
                >
                  <option value="manual">DNS 레코드 직접 등록</option>
                  <option value="cloudflare">Cloudflare 자동 등록</option>
                  <option
                    value="domainconnect"
                    disabled={
                      !providers.data?.providers.find(
                        (p) => p.id === "domainconnect",
                      )?.supported
                    }
                  >
                    Domain Connect · 지원 DNS 서비스
                  </option>
                </select>
              </Field>
              <Field
                label="연결 대상 (선택)"
                hint="비워두면 이 아티팩트의 기본 콘텐츠 주소를 사용합니다."
              >
                <input
                  value={form.target}
                  onChange={(e) => setForm({ ...form, target: e.target.value })}
                  placeholder="content.example.internal"
                />
              </Field>
              <Notice>
                다음 단계에서 소유권 확인용 TXT와 연결 레코드를 안내합니다. 기존
                DNS 레코드는 자동으로 덮어쓰지 않습니다.
              </Notice>
              <ErrorBox error={a.error} />
            </div>
            <footer className="dialog-footer">
              <button
                type="button"
                className="button secondary"
                onClick={() => setShow(false)}
              >
                취소
              </button>
              <button className="button primary" disabled={a.busy}>
                {a.busy ? <Spinner /> : "다음: DNS 등록"}
              </button>
            </footer>
          </form>
        </Modal>
      )}
    </>
  );
}

export function AgentsPage() {
  const r = useResource<{ tokens: Token[] }>("/api/tokens");
  const [name, setName] = useState("");
  const [expires, setExpires] = useState("");
  const [scopes, setScopes] = useState(["read", "publish"]);
  const [secret, setSecret] = useState("");
  const a = useAction();
  const mcp = JSON.stringify(
    {
      mcpServers: {
        folio: {
          command: "folio",
          args: ["mcp"],
          env: { FOLIO_URL: location.origin, FOLIO_TOKEN: "YOUR_API_TOKEN" },
        },
      },
    },
    null,
    2,
  );
  return (
    <>
      <PageHeading
        eyebrow="WORKSPACE / AGENTS"
        title="에이전트에게도 같은 작업 공간을"
        description="모델이나 구독에 종속되지 않는 MCP 도구와 CLI를 연결하세요."
      />
      <ErrorBox error={r.error || a.error} />
      <div className="agent-intro">
        <div className="agent-glyph">
          <Terminal size={30} />
        </div>
        <div>
          <h2>만들기는 에이전트에게, 공유는 Folio로.</h2>
          <p>
            파일과 폴더를 게시하고, 버전을 갱신하고, 공유할 사람을 지정합니다.
            <br />
            에이전트도 계정에 허용된 범위 안에서만 작업합니다.
          </p>
        </div>
      </div>
      <div className="two-columns">
        <section className="panel">
          <div className="section-title">
            <h2>1. API 토큰 발급</h2>
            <KeyRound size={19} />
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void a.run(async () => {
                const result = await api<{ token: string }>(
                  "/api/tokens",
                  json("POST", {
                    name,
                    scopes,
                    expiresAt: expires ? new Date(expires).toISOString() : null,
                  }),
                );
                setSecret(result.token);
                setName("");
                r.reload();
              });
            }}
          >
            <Field label="토큰 이름">
              <input
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="예: 내 개발 에이전트"
              />
            </Field>
            <fieldset className="scope-checks">
              <legend>허용할 작업</legend>
              {[
                { id: "read", label: "목록·미리보기" },
                { id: "publish", label: "게시·버전 갱신·보관" },
                { id: "share", label: "공유 설정 변경" },
              ].map((x) => (
                <label key={x.id}>
                  <input
                    type="checkbox"
                    checked={scopes.includes(x.id)}
                    onChange={(e) =>
                      setScopes(
                        e.target.checked
                          ? [...scopes, x.id]
                          : scopes.filter((s) => s !== x.id),
                      )
                    }
                  />
                  {x.label}
                </label>
              ))}
            </fieldset>
            <Field label="만료일 (선택)">
              <input
                type="datetime-local"
                value={expires}
                onChange={(e) => setExpires(e.target.value)}
              />
            </Field>
            <button
              className="button primary"
              disabled={a.busy || !scopes.length}
            >
              {a.busy ? <Spinner /> : "토큰 발급"}
            </button>
          </form>
          {secret && (
            <div className="secret-box vertical">
              <b>한 번만 표시하는 API 토큰</b>
              <code>{secret}</code>
              <CopyButton text={secret} />
              <small>안전하게 보관한 후 이 표시를 닫아주세요.</small>
              <button
                className="button secondary compact"
                onClick={() => setSecret("")}
              >
                토큰 표시 닫기
              </button>
            </div>
          )}
        </section>
        <section className="panel">
          <div className="section-title">
            <h2>2. MCP 연결</h2>
            <Server size={19} />
          </div>
          <p className="subtle">
            Folio CLI를 설치한 뒤 에이전트의 MCP 설정에 추가하세요. 토큰을 공개
            저장소에 넣지 마세요.
          </p>
          <pre className="code-block">{mcp}</pre>
          <CopyButton text={mcp} label="설정 복사" />
          <h3 className="mt">CLI로 바로 게시</h3>
          <pre className="code-block">
            {"folio login --token-stdin --url " +
              location.origin +
              "\nfolio publish ./my-artifact\nfolio mcp"}
          </pre>
          <p className="subtle">
            지원 도구: 아티팩트 목록·검색·게시·수정·공유·보관·복원·한도 확인
          </p>
        </section>
      </div>
      <section className="panel">
        <div className="section-title">
          <h2>내 API 토큰</h2>
          <span className="subtle">
            폐기하면 연결된 에이전트의 접근도 해제됩니다.
          </span>
        </div>
        {!r.data?.tokens.length ? (
          <p className="muted-padded">아직 발급한 토큰이 없습니다.</p>
        ) : (
          r.data.tokens.map((t) => (
            <div className="connection-row" key={t.id}>
              <KeyRound size={20} />
              <div className="connection-name">
                <b>{t.name}</b>
                <small>
                  {t.scopes.join(" · ")} · {date(t.createdAt)} 발급
                  {t.expiresAt ? ` · ${date(t.expiresAt)} 만료` : ""}
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
                폐기
              </button>
            </div>
          ))
        )}
      </section>
    </>
  );
}

export function SettingsPage({ onChanged }: { onChanged: () => void }) {
  const r = useResource<{
    organization: Organization;
    policy: Policy;
    appUrl: string;
    contentUrl: string;
  }>("/api/admin/settings");
  const [name, setName] = useState("");
  const [saved, setSaved] = useState(false);
  const a = useAction();
  useEffect(() => {
    if (r.data) setName(r.data.organization.name);
  }, [r.data]);
  return (
    <>
      <PageHeading
        eyebrow="WORKSPACE / SETTINGS"
        title="우리 팀의 작업 공간"
        description="조직 이름과 설치 정보를 확인합니다."
      />
      <ErrorBox error={r.error || a.error} />
      <section className="panel">
        <h2>조직 정보</h2>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void a.run(async () => {
              await api(
                "/api/admin/settings",
                json("PATCH", { organizationName: name }),
              );
              setSaved(true);
              r.reload();
              onChanged();
            });
          }}
        >
          <Field label="조직 이름">
            <input
              required
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setSaved(false);
              }}
              maxLength={100}
            />
          </Field>
          <button className="button primary" disabled={a.busy}>
            {a.busy ? <Spinner /> : "변경 사항 저장"}
          </button>
          {saved && (
            <span className="saved-inline">
              <CheckCircle2 size={15} />
              저장했습니다.
            </span>
          )}
        </form>
      </section>
      <section className="panel">
        <h2>설치 정보</h2>
        <dl className="detail-list">
          <div>
            <dt>관리 주소</dt>
            <dd>{r.data?.appUrl}</dd>
          </div>
          <div>
            <dt>아티팩트 주소</dt>
            <dd>{r.data?.contentUrl}</dd>
          </div>
          <div>
            <dt>배포 방식</dt>
            <dd>설치형 · 단일 조직</dd>
          </div>
          <div>
            <dt>라이선스</dt>
            <dd>MIT · Open source</dd>
          </div>
        </dl>
        <Notice>
          아티팩트는 관리 페이지와 분리된 도메인에서 실행합니다. 주소 변경은
          서버 설정에서 적용해 주세요.
        </Notice>
      </section>
    </>
  );
}
export function AuditPage() {
  const r = useResource<{ events: Audit[] }>("/api/admin/audit?limit=100");
  return (
    <>
      <PageHeading
        eyebrow="WORKSPACE / AUDIT"
        title="작업 공간의 기록"
        description="계정, 공유, 게시와 보관 정책의 변경을 확인합니다."
        action={
          <button className="button secondary" onClick={r.reload}>
            <RefreshCw size={16} />
            새로고침
          </button>
        }
      />
      <ErrorBox error={r.error} />
      <section className="panel">
        {r.loading ? (
          <div className="loading-row">
            <Spinner />
          </div>
        ) : !r.data?.events.length ? (
          <Empty
            icon={<ShieldCheck size={30} />}
            heading="아직 기록이 없습니다"
            description="작업 공간에서 일어난 변경 사항이 여기에 남습니다."
          />
        ) : (
          <div className="audit-list">
            {r.data.events.map((e) => (
              <div className="audit-row" key={e.id}>
                <span className="audit-dot" />
                <div>
                  <b>{e.action}</b>
                  <p>
                    {e.actorName || "시스템"}
                    {e.detail ? ` · ${e.detail}` : ""}
                  </p>
                </div>
                <time>{date(e.createdAt)}</time>
              </div>
            ))}
          </div>
        )}
      </section>
    </>
  );
}

export function AccountPage({
  session,
  onSession,
}: {
  session: Session;
  onSession: (s: Session) => void;
}) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [saved, setSaved] = useState(false);
  const a = useAction();
  return (
    <>
      <PageHeading
        eyebrow="ACCOUNT / PROFILE"
        title="내 계정"
        description="계정 정보를 확인하고 비밀번호를 변경합니다."
      />
      <section className="panel">
        <h2>계정 정보</h2>
        <dl className="detail-list">
          <div>
            <dt>이름</dt>
            <dd>{session.user.name}</dd>
          </div>
          <div>
            <dt>이메일</dt>
            <dd>{session.user.email}</dd>
          </div>
          <div>
            <dt>역할</dt>
            <dd>{session.user.role === "admin" ? "관리자" : "구성원"}</dd>
          </div>
        </dl>
      </section>
      <section className="panel">
        <h2>비밀번호 변경</h2>
        <p className="subtle">
          변경하면 다른 로그인 세션과 기존 API 토큰이 해제됩니다.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void a.run(async () => {
              if (next !== confirm)
                throw new Error("새 비밀번호가 일치하지 않습니다.");
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
            <Field label="현재 비밀번호">
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
            <Field label="새 비밀번호">
              <input
                required
                type="password"
                minLength={12}
                autoComplete="new-password"
                value={next}
                onChange={(e) => setNext(e.target.value)}
              />
            </Field>
            <Field label="새 비밀번호 확인">
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
            {a.busy ? <Spinner /> : "비밀번호 변경"}
          </button>
          {saved && (
            <span className="saved-inline">
              <CheckCircle2 size={15} />
              비밀번호를 변경했습니다.
            </span>
          )}
        </form>
      </section>
    </>
  );
}
