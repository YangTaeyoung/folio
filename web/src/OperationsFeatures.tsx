import { Select } from "./Select";
import { useCallback, useEffect, useState } from "react";
import {
  ArchiveRestore,
  Download,
  Link2,
  LockKeyhole,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { Brand } from "./Auth";
import { api, APIError, bytes, date, json } from "./api";
import { CopyButton, ErrorBox, Field, Spinner } from "./ui";
import { t, useLocale } from "./i18n";
import type { Artifact, Version } from "./types";
import "./OperationsFeatures.css";

interface TrashArtifact extends Artifact {
  deletedAt: string;
  deletedBy: string;
}
interface GuestLink {
  id: string;
  versionId: string;
  url: string;
  expiresAt: string;
  revokedAt?: string;
  passwordProtected: boolean;
}
interface Backup {
  id: string;
  createdAt: string;
  sizeBytes: number;
}
interface GuestMetadata {
  id: string;
  passwordProtected: boolean;
  unlocked: boolean;
  expiresAt: string;
  artifact?: Artifact;
  versionId?: string;
  csrfToken?: string;
}

export function TrashView({ onChange }: { onChange?: () => void }) {
  useLocale();
  const [items, setItems] = useState<TrashArtifact[]>([]),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(""),
    [error, setError] = useState<unknown>(null),
    [confirm, setConfirm] = useState("");
  const load = useCallback(async (signal?: AbortSignal) => {
    const out = await api<{ artifacts: TrashArtifact[] }>("/api/trash", {
      signal,
    });
    if (!signal?.aborted) setItems(out.artifacts);
  }, []);
  useEffect(() => {
    const c = new AbortController();
    void load(c.signal)
      .catch((e) => {
        if (!c.signal.aborted) setError(e);
      })
      .finally(() => {
        if (!c.signal.aborted) setLoading(false);
      });
    return () => c.abort();
  }, [load]);
  async function act(id: string, purge: boolean) {
    setBusy(id);
    setError(null);
    try {
      await api(
        `/api/trash/${id}${purge ? "" : "/restore"}`,
        json(purge ? "DELETE" : "POST"),
      );
      setConfirm("");
      await load();
      onChange?.();
    } catch (e) {
      setError(e);
    } finally {
      setBusy("");
    }
  }
  return (
    <section className="ops-section">
      <header className="ops-heading">
        <div>
          <h2>{t("휴지통")}</h2>
          <p>
            {t(
              "삭제한 아티팩트는 30일 동안 복원할 수 있습니다. 보관된 항목은 자동으로 영구 삭제되지 않습니다. 기존 게스트 링크는 해제됩니다.",
            )}
          </p>
        </div>
        <button
          className="button secondary"
          onClick={() => {
            setError(null);
            void load().catch(setError);
          }}
          aria-label={t("새로고침")}
        >
          <RefreshCw size={16} />
        </button>
      </header>
      <ErrorBox error={error} />
      {loading ? (
        <Spinner />
      ) : items.length === 0 ? (
        <div className="ops-empty">
          <Trash2 size={24} />
          <h3>{t("휴지통이 비어 있습니다")}</h3>
          <p>{t("삭제한 아티팩트가 여기에 표시됩니다.")}</p>
        </div>
      ) : (
        <ul className="ops-list">
          {items.map((a) => (
            <li key={a.id}>
              <div className="ops-item-main">
                <strong>{a.title}</strong>
                <small>
                  {date(a.deletedAt)} · {bytes(a.sizeBytes)}
                  {a.archived ? ` · ${t("보관됨")}` : ""}
                </small>
              </div>
              <div className="ops-actions">
                <button
                  disabled={!!busy}
                  className="button secondary"
                  onClick={() => void act(a.id, false)}
                >
                  <ArchiveRestore size={16} />
                  {t("복원")}
                </button>
                {confirm === a.id ? (
                  <>
                    <span className="ops-warning">
                      {t("복원할 수 없습니다. 영구 삭제할까요?")}
                    </span>
                    <button
                      disabled={!!busy}
                      className="button danger"
                      onClick={() => void act(a.id, true)}
                    >
                      {t("영구 삭제")}
                    </button>
                    <button
                      className="button secondary"
                      disabled={!!busy}
                      onClick={() => setConfirm("")}
                    >
                      {t("취소")}
                    </button>
                  </>
                ) : (
                  <button
                    disabled={!!busy}
                    className="button secondary"
                    onClick={() => setConfirm(a.id)}
                  >
                    {t("영구 삭제")}
                  </button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

export function GuestLinks({
  artifact,
  versions,
}: {
  artifact: Artifact;
  versions: Version[];
}) {
  useLocale();
  const [links, setLinks] = useState<GuestLink[]>([]),
    [version, setVersion] = useState(artifact.currentVersionId),
    [expiry, setExpiry] = useState(""),
    [password, setPassword] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState<unknown>(null),
    [loaded, setLoaded] = useState(false);
  const base = `/api/artifacts/${encodeURIComponent(artifact.id)}/guest-links`;
  const load = useCallback(
    async (signal?: AbortSignal) => {
      const out = await api<{ guestLinks: GuestLink[] }>(base, { signal });
      if (!signal?.aborted) {
        setLinks(out.guestLinks);
        setLoaded(true);
      }
    },
    [base],
  );
  useEffect(() => {
    const c = new AbortController();
    setLinks([]);
    setLoaded(false);
    setVersion(artifact.currentVersionId);
    setPassword("");
    setError(null);
    void load(c.signal).catch((e) => {
      if (!c.signal.aborted) setError(e);
    });
    return () => c.abort();
  }, [load, artifact.currentVersionId]);
  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api(
        base,
        json("POST", {
          versionId: version,
          password,
          expiresAt: new Date(expiry).toISOString(),
        }),
      );
      setPassword("");
      await load();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  async function revoke(id: string) {
    setBusy(true);
    setError(null);
    try {
      await api(`${base}/${id}/revoke`, json("POST"));
      await load();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="ops-section">
      <header className="ops-heading">
        <div>
          <h3>{t("게스트 링크")}</h3>
          <p>
            {t(
              "선택한 버전만 읽을 수 있는 링크입니다. 만료일과 비밀번호를 설정하고 언제든 해제할 수 있습니다.",
            )}
          </p>
        </div>
        <Link2 size={18} />
      </header>
      <ErrorBox error={error} />
      <form className="ops-form" onSubmit={create}>
        <Field label={t("공유할 버전")}>
          <Select
            value={version}
            onValueChange={(value) => setVersion(value)}
            required
          >
            {versions.map((v) => (
              <option key={v.id} value={v.id}>
                {t("버전 {number}", { number: v.number })} · {date(v.createdAt)}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={t("만료일")}>
          <input
            type="datetime-local"
            value={expiry}
            required
            onChange={(e) => setExpiry(e.target.value)}
          />
        </Field>
        <Field
          label={t("비밀번호 (선택)")}
          hint={t("12~72바이트. 비밀번호는 링크에 포함되지 않습니다.")}
        >
          <input
            type="password"
            autoComplete="new-password"
            minLength={12}
            maxLength={72}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>
        <button className="button primary" disabled={busy || !version}>
          {busy ? <Spinner /> : <Link2 size={16} />} {t("게스트 링크 만들기")}
        </button>
      </form>
      {!loaded && !error ? (
        <Spinner />
      ) : (
        <ul className="ops-list">
          {links.map((link) => (
            <li key={link.id}>
              <div className="ops-item-main">
                <span>
                  {versions.find((v) => v.id === link.versionId)
                    ? t("버전 {number}", {
                        number: versions.find((v) => v.id === link.versionId)!
                          .number,
                      })
                    : t("선택한 버전")}
                  {link.passwordProtected && (
                    <LockKeyhole size={14} aria-label={t("비밀번호 보호")} />
                  )}
                </span>
                <small>
                  {link.revokedAt
                    ? t("해제됨")
                    : new Date(link.expiresAt).getTime() <= Date.now()
                      ? t("만료됨")
                      : t("{date}에 만료", { date: date(link.expiresAt) })}
                </small>
              </div>
              {!link.revokedAt &&
                new Date(link.expiresAt).getTime() > Date.now() && (
                  <div className="ops-actions">
                    <CopyButton text={link.url} label={t("링크 복사")} />
                    <button
                      type="button"
                      className="button secondary"
                      disabled={busy}
                      onClick={() => void revoke(link.id)}
                    >
                      {t("링크 해제")}
                    </button>
                  </div>
                )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

export function BackupSettings() {
  useLocale();
  const [backups, setBackups] = useState<Backup[]>([]),
    [password, setPassword] = useState(""),
    [restorePassword, setRestorePassword] = useState(""),
    [file, setFile] = useState<File | null>(null),
    [confirmed, setConfirmed] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState<unknown>(null),
    [notice, setNotice] = useState("");
  const load = useCallback(async (signal?: AbortSignal) => {
    const out = await api<{ backups: Backup[] }>("/api/backups", { signal });
    if (!signal?.aborted) setBackups(out.backups);
  }, []);
  useEffect(() => {
    const c = new AbortController();
    void load(c.signal).catch((e) => {
      if (!c.signal.aborted) setError(e);
    });
    return () => c.abort();
  }, [load]);
  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setNotice("");
    try {
      await api("/api/backups", json("POST", { password }));
      setPassword("");
      await load();
      setNotice(t("암호화된 백업을 만들었습니다. 아래에서 다운로드하세요."));
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  async function restore(e: React.FormEvent) {
    e.preventDefault();
    if (!file || !confirmed) return;
    setBusy(true);
    setError(null);
    setNotice("");
    try {
      await api("/api/backups/restore", {
        method: "POST",
        body: file,
        headers: {
          "X-Backup-Password": btoa(
            String.fromCharCode(...new TextEncoder().encode(restorePassword)),
          ),
          "X-Backup-Password-Encoding": "base64",
        },
      });
      setRestorePassword("");
      setConfirmed(false);
      setNotice(
        t(
          "백업을 복원했습니다. 새로고침하면 복원된 아티팩트를 볼 수 있습니다.",
        ),
      );
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="ops-section">
      <header className="ops-heading">
        <div>
          <h2>{t("백업과 복원")}</h2>
          <p>
            {t(
              "모든 버전의 파일과 메타데이터를 비밀번호로 암호화합니다. 외부 저장소의 파일도 포함됩니다.",
            )}
          </p>
        </div>
      </header>
      <ErrorBox error={error} />
      {notice && (
        <p role="status" className="ops-notice">
          {t(notice)}
        </p>
      )}
      <form className="ops-form" onSubmit={create}>
        <Field
          label={t("백업 비밀번호")}
          hint={t("12자 이상. 비밀번호를 잊으면 백업을 복원할 수 없습니다.")}
        >
          <input
            type="password"
            autoComplete="new-password"
            required
            minLength={12}
            maxLength={256}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>
        <button className="button primary" disabled={busy}>
          {busy ? <Spinner /> : <LockKeyhole size={16} />}{" "}
          {t("암호화 백업 만들기")}
        </button>
      </form>
      <ul className="ops-list">
        {backups.map((b) => (
          <li key={b.id}>
            <div className="ops-item-main">
              <strong>{date(b.createdAt)}</strong>
              <small>{bytes(b.sizeBytes)}</small>
            </div>
            <a
              className="button secondary"
              href={`/api/backups/${b.id}`}
              download
            >
              <Download size={16} />
              {t("다운로드")}
            </a>
          </li>
        ))}
      </ul>
      <div className="ops-restore">
        <h3>{t("백업에서 복원")}</h3>
        <p>
          {t(
            "현재 아티팩트 목록을 백업으로 교체합니다. 현재 계정과 로그인·Google·DNS 설정을 유지하고, 파일은 새 로컬 저장소에 복원합니다. 게스트 링크와 API 토큰은 다시 만들어야 합니다.",
          )}
        </p>
        <p>{t("최대 512MB. 복원 전에 현재 데이터의 백업을 다운로드하세요.")}</p>
        <form className="ops-form" onSubmit={restore}>
          <Field label={t("백업 파일")}>
            <input
              type="file"
              accept=".folio"
              required
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </Field>
          <Field label={t("백업 비밀번호")}>
            <input
              type="password"
              autoComplete="off"
              required
              minLength={12}
              maxLength={256}
              value={restorePassword}
              onChange={(e) => setRestorePassword(e.target.value)}
            />
          </Field>
          <label className="ops-check">
            <input
              type="checkbox"
              checked={confirmed}
              onChange={(e) => setConfirmed(e.target.checked)}
            />
            {t("현재 아티팩트 목록이 백업으로 교체되는 것을 확인했습니다.")}
          </label>
          <button
            className="button danger"
            disabled={busy || !file || !confirmed}
          >
            {busy ? <Spinner /> : <ArchiveRestore size={16} />} {t("백업 복원")}
          </button>
        </form>
      </div>
    </section>
  );
}

export function GuestViewer({ id }: { id: string }) {
  useLocale();
  const [meta, setMeta] = useState<GuestMetadata | null>(null),
    [password, setPassword] = useState(""),
    [error, setError] = useState<unknown>(null),
    [busy, setBusy] = useState(false),
    [url, setURL] = useState("");
  const base = `/api/guest/${encodeURIComponent(id)}`;
  useEffect(() => {
    const c = new AbortController();
    setMeta(null);
    setURL("");
    setError(null);
    void api<GuestMetadata>(base, { signal: c.signal })
      .then((out) => {
        if (!c.signal.aborted) setMeta(out);
      })
      .catch((e) => {
        if (!c.signal.aborted) setError(e);
      });
    return () => c.abort();
  }, [base]);
  async function open(e?: React.FormEvent) {
    e?.preventDefault();
    setBusy(true);
    setError(null);
    try {
      let token = meta?.csrfToken;
      if (!meta?.unlocked) {
        const unlocked = await api<GuestMetadata>(
          `${base}/unlock`,
          json("POST", { password }),
        );
        token = unlocked.csrfToken;
        setMeta((old) =>
          old ? { ...old, ...unlocked, unlocked: true } : unlocked,
        );
        setPassword("");
      }
      const headers = { "X-CSRF-Token": token ?? "" };
      const res = await fetch(`${base}/launch`, {
        ...json("POST", {}),
        headers: { ...headers, "Content-Type": "application/json" },
        credentials: "same-origin",
      });
      const out = await res.json();
      if (!res.ok)
        throw new APIError(
          out.error?.message || t("게스트 링크를 열지 못했습니다."),
          res.status,
          out.error?.code || "request_failed",
        );
      setURL(out.url);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className={`ops-guest ${url ? "ops-guest-open" : ""}`}>
      <header>
        <a href="/" className="ops-wordmark">
          <Brand />
        </a>
        <span>{t("읽기 전용 게스트 링크")}</span>
      </header>
      <ErrorBox error={error} />
      {url ? (
        <>
          <div className="ops-guest-toolbar">
            <strong>{meta?.artifact?.title}</strong>
            <button
              type="button"
              className="button secondary"
              disabled={busy}
              onClick={() => void open()}
            >
              {t("새로고침")}
            </button>
          </div>
          <iframe
            title={meta?.artifact?.title || t("게스트 아티팩트")}
            src={url}
            sandbox="allow-scripts allow-forms allow-downloads allow-same-origin"
            referrerPolicy="no-referrer"
          />
        </>
      ) : meta ? (
        <section className="ops-guest-gate">
          <LockKeyhole size={28} />
          <h1>{meta.artifact?.title || t("공유된 아티팩트")}</h1>
          <p>
            {meta.passwordProtected && !meta.unlocked
              ? t("비밀번호를 입력해 선택한 버전을 확인하세요.")
              : t("이 링크는 지정된 버전만 읽을 수 있습니다.")}
          </p>
          <form onSubmit={open}>
            {meta.passwordProtected && !meta.unlocked && (
              <Field label={t("비밀번호")}>
                <input
                  autoFocus
                  type="password"
                  autoComplete="current-password"
                  required
                  maxLength={72}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </Field>
            )}
            <button className="button primary" disabled={busy}>
              {busy ? <Spinner /> : null}
              {t("아티팩트 열기")}
            </button>
          </form>
          <small>{t("{date}에 만료", { date: date(meta.expiresAt) })}</small>
        </section>
      ) : !error ? (
        <Spinner />
      ) : (
        <p>
          {t(
            "링크가 만료되었거나 해제되었습니다. 공유한 사람에게 새 링크를 요청하세요.",
          )}
        </p>
      )}
    </main>
  );
}
