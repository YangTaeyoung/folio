import { Select } from "./Select";
import { ReviewPanel } from "./ReviewPanel";
import { ArtifactContext, VersionCompare } from "./WorkspaceFeatures";
import { GuestLinks } from "./OperationsFeatures";
import { t as translate, useLocale } from "./i18n";
import { useEffect, useRef, useState } from "react";
import {
  Archive,
  ArrowUpRight,
  BookOpen,
  Download,
  FileCode,
  FileText,
  Globe,
  History,
  Link,
  Lock,
  Plus,
  RotateCcw,
  Search,
  Share2,
  Trash2,
  UploadCloud,
  Users,
} from "lucide-react";
import { api, bytes, date, json, relative } from "./api";
import type { Artifact, User, Version, Visibility } from "./types";
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

export const visibilityLabels: Record<Visibility, string> = {
  get private() {
    return translate("나만 보기");
  },
  get organization() {
    return translate("조직 공유");
  },
  get selected() {
    return translate("선택한 사람");
  },
  get public() {
    return translate("전체 공개");
  },
};
/** Keep an unpublished version explicit; the stable share URL still opens the published version. */
export function artifactOpenURL(artifact: Artifact) {
  if (
    !artifact.reviewEnabled ||
    artifact.currentVersionId === artifact.publishedVersionId
  )
    return artifact.shareUrl;
  const url = new URL(artifact.shareUrl, window.location.origin);
  url.searchParams.set("version", artifact.currentVersionId);
  return url.toString();
}
export function VisibilityIcon({
  value,
  size = 14,
}: {
  value: Visibility;
  size?: number;
}) {
  useLocale();
  const Icon = value === "private" ? Lock : value === "public" ? Globe : Users;
  return <Icon size={size} />;
}
export function ArtifactPreview({
  artifact,
  className = "",
  isolatedVersion = false,
}: {
  artifact: Artifact;
  isolatedVersion?: boolean;
  className?: string;
}) {
  useLocale();
  const [url, setURL] = useState("");
  const [error, setError] = useState("");
  const [loaded, setLoaded] = useState(false);
  const target = useRef<HTMLDivElement>(null);
  const [previewWidth, setPreviewWidth] = useState(0);
  const interactive = className.split(/\s+/).includes("large");
  useEffect(() => {
    if (interactive || !target.current) return;
    const element = target.current;
    setPreviewWidth(element.clientWidth);
    const observer = new ResizeObserver(([entry]) => {
      if (entry) setPreviewWidth(entry.contentRect.width);
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [interactive]);
  useEffect(() => {
    let active = true;
    setURL("");
    setError("");
    setLoaded(false);
    const launch = () => {
      api<{ url: string }>(
        `/api/artifacts/${artifact.id}/launch`,
        json("POST", { versionId: artifact.currentVersionId, isolatedVersion }),
      )
        .then((r) => {
          if (active) setURL(r.url);
        })
        .catch((e) => {
          if (active) setError(e.message);
        });
    };
    let observer: IntersectionObserver | undefined;
    if (target.current && typeof IntersectionObserver !== "undefined") {
      observer = new IntersectionObserver(
        (entries) => {
          if (entries.some((entry) => entry.isIntersecting)) {
            observer?.disconnect();
            launch();
          }
        },
        { rootMargin: "250px" },
      );
      observer.observe(target.current);
    } else launch();
    return () => {
      active = false;
      observer?.disconnect();
    };
  }, [
    artifact.id,
    artifact.currentVersionId,
    artifact.publishedVersionId,
    isolatedVersion,
  ]);
  return (
    <div
      ref={target}
      className={`artifact-preview ${className}`}
      aria-busy={!error && !loaded}
    >
      {url && !loaded && (
        <div className="preview-loading">
          <Spinner />
        </div>
      )}
      {url ? (
        <iframe
          title={translate("{p0} 미리보기", { p0: artifact.title })}
          src={url}
          style={
            interactive
              ? undefined
              : {
                  width: 1024,
                  height: 640,
                  transform: `scale(${previewWidth / 1024})`,
                }
          }
          onLoad={() => setLoaded(true)}
          sandbox="allow-scripts allow-same-origin allow-forms allow-downloads"
        />
      ) : (
        <div className="preview-placeholder">
          <FileText size={28} />
          <span>
            {error
              ? translate("미리보기를 불러오지 못했습니다.")
              : artifact.kind?.toUpperCase() || "ARTIFACT"}
          </span>
        </div>
      )}
    </div>
  );
}
export function ArtifactCard({
  artifact,
  onShare,
}: {
  artifact: Artifact;
  onOpen?: () => void;
  onShare: () => void;
}) {
  useLocale();
  return (
    <article className="artifact-card">
      <a
        className="artifact-open"
        href={artifactOpenURL(artifact)}
        aria-label={translate("{p0} 열기", { p0: artifact.title })}
      >
        <ArtifactPreview artifact={artifact} />
        <h3>
          {artifact.title}
          {artifact.reviewEnabled &&
            artifact.currentVersionId !== artifact.publishedVersionId && (
              <span className="artifact-version-state">
                {translate("초안")}
              </span>
            )}
        </h3>
        <p
          className={`artifact-description ${artifact.searchSnippet ? "artifact-snippet" : ""}`}
        >
          {artifact.searchSnippet ||
            artifact.description ||
            translate("설명 없음")}
        </p>
      </a>
      <div className="artifact-meta">
        <span className="owner-name">
          <Avatar
            name={artifact.ownerName || "F"}
            src={artifact.ownerAvatarUrl}
          />
          <span>
            <span>{artifact.ownerName || "—"}</span>
            <small>
              {translate("{p0} 수정", { p0: relative(artifact.updatedAt) })}
            </small>
          </span>
        </span>
        <span className={`scope-label scope-${artifact.visibility}`}>
          <VisibilityIcon value={artifact.visibility} />
          {visibilityLabels[artifact.visibility]}
        </span>
        <button
          className="icon-button artifact-share"
          onClick={onShare}
          aria-label={translate("{p0} 공유", { p0: artifact.title })}
          title={translate("공유 설정")}
        >
          <Share2 size={16} />
        </button>
      </div>
    </article>
  );
}

export function SharePanel({
  artifact,
  onClose,
  onSaved,
  canManage = true,
}: {
  artifact: Artifact;
  onClose: () => void;
  onSaved: () => void;
  canManage?: boolean;
}) {
  useLocale();
  const [guestVersions, setGuestVersions] = useState<Version[]>([]);
  useEffect(() => {
    if (!canManage) return;
    let active = true;
    api<{ versions: Version[] }>(`/api/artifacts/${artifact.id}`)
      .then((r) => {
        if (active) setGuestVersions(r.versions);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [artifact.id, canManage]);
  const [visibility, setVisibility] = useState(artifact.visibility);
  const [ids, setIDs] = useState<string[]>(artifact.memberIds || []);
  const [expires, setExpires] = useState(() =>
    artifact.expiresAt
      ? new Date(
          new Date(artifact.expiresAt).getTime() -
            new Date().getTimezoneOffset() * 60000,
        )
          .toISOString()
          .slice(0, 16)
      : "",
  );
  const [members, setMembers] = useState<User[]>([]);
  const [q, setQ] = useState("");
  const a = useAction();
  const sameExpiry =
    (!expires && !artifact.expiresAt) ||
    (expires &&
      artifact.expiresAt &&
      new Date(expires).getTime() === new Date(artifact.expiresAt).getTime());
  const changed =
    visibility !== artifact.visibility ||
    !sameExpiry ||
    (visibility === "selected" &&
      [...ids].sort().join(",") !==
        [...(artifact.memberIds || [])].sort().join(","));
  useEffect(() => {
    api<{ members: User[] }>("/api/members")
      .then((r) => setMembers(r.members || []))
      .catch(a.setError);
  }, []);
  return (
    <Modal
      title={translate("아티팩트 공유")}
      drawer
      className="artifact-sharing"
      onClose={onClose}
    >
      <div className="drawer-body">
        <div className="drawer-document">
          <FileText size={23} />
          <div>
            <b>{artifact.title}</b>
            <small>
              {artifact.ownerName} · {relative(artifact.updatedAt)}{" "}
              {translate("수정")}
            </small>
          </div>
        </div>
        <div className="share-current-access">
          <span>{translate("현재 공유 범위")}</span>
          <span className={`scope-label scope-${artifact.visibility}`}>
            <VisibilityIcon value={artifact.visibility} />
            {visibilityLabels[artifact.visibility]}
          </span>
        </div>
        <fieldset className="visibility-options">
          <legend>{translate("공유 범위")}</legend>
          {(
            [
              {
                value: "private",
                label: translate("나만 보기"),
                description: translate("작성자와 관리자만 접근"),
              },
              {
                value: "organization",
                label: translate("조직 구성원"),
                description: translate("로그인한 모든 조직 구성원"),
              },
              {
                value: "selected",
                label: translate("선택한 구성원"),
                description: translate("지정한 구성원에게만 공유"),
              },
              {
                value: "public",
                label: translate("링크가 있는 모든 사람"),
                description: translate("로그인 없이 누구나 열람"),
              },
            ] as const
          ).map((option) => (
            <label
              className={`visibility-option ${visibility === option.value ? "selected" : ""}`}
              key={option.value}
            >
              <VisibilityIcon value={option.value} size={19} />
              <span>
                <b>{option.label}</b>
                <small>{option.description}</small>
              </span>
              <input
                type="radio"
                name="visibility"
                value={option.value}
                checked={visibility === option.value}
                disabled={!canManage}
                onChange={() => setVisibility(option.value)}
              />
            </label>
          ))}
        </fieldset>
        {visibility === "selected" && (
          <section className="share-people">
            <label className="field">
              <span>{translate("함께 볼 사람 선택")}</span>
              <div className="input-icon">
                <Search size={17} />
                <input
                  value={q}
                  onChange={(e) => setQ(e.target.value)}
                  placeholder={translate("이름 또는 이메일 검색")}
                />
              </div>
            </label>
            <div className="people-options">
              {members
                .filter(
                  (u) =>
                    !u.disabled &&
                    u.id !== artifact.ownerId &&
                    `${u.name} ${u.email}`
                      .toLowerCase()
                      .includes(q.toLowerCase()),
                )
                .map((u) => (
                  <label className="person-option" key={u.id}>
                    <input
                      type="checkbox"
                      aria-label={translate("{p0} 선택", { p0: u.name })}
                      disabled={!canManage}
                      checked={ids.includes(u.id)}
                      onChange={(e) =>
                        setIDs(
                          e.target.checked
                            ? [...ids, u.id]
                            : ids.filter((id) => id !== u.id),
                        )
                      }
                    />
                    <Avatar name={u.name} src={u.avatarUrl} />
                    <span>
                      <b>{u.name}</b>
                      <small>{u.email}</small>
                    </span>
                    <span className="person-role">{translate("열람")}</span>
                  </label>
                ))}
              {!members.length && (
                <p className="subtle">
                  {translate("아직 선택할 구성원이 없습니다.")}
                </p>
              )}
            </div>
          </section>
        )}
        <div className="access-note">
          <VisibilityIcon value={visibility} size={18} />
          <p>
            {visibility === "private"
              ? translate("작성자와 관리자만 열 수 있습니다.")
              : visibility === "organization"
                ? translate("로그인한 조직 구성원이 열 수 있습니다.")
                : visibility === "selected"
                  ? translate("선택한 구성원과 작성자·관리자만 열 수 있습니다.")
                  : translate(
                      "링크를 전달받은 누구나 로그인 없이 열 수 있습니다.",
                    )}
          </p>
        </div>
        <Field
          label={translate("공유 만료")}
          hint={translate(
            "만료 시간이 지나면 작성자와 관리자만 볼 수 있습니다.",
          )}
        >
          <input
            disabled={!canManage}
            aria-label={translate("공유 만료")}
            type="datetime-local"
            value={expires}
            onChange={(e) => setExpires(e.target.value)}
          />
        </Field>
        <section className="share-link">
          <label>{translate("공유 링크")}</label>
          <div className="link-field">
            <Link size={16} />
            <input
              readOnly
              value={
                artifact.shareUrl || `${location.origin}/s/${artifact.slug}`
              }
              aria-label={translate("공유 링크")}
            />
          </div>
          <CopyButton
            text={artifact.shareUrl || `${location.origin}/s/${artifact.slug}`}
            label={translate("링크 복사")}
          />
        </section>
        <ErrorBox error={a.error} />
        {canManage && (
          <div className="guest-share-section">
            <GuestLinks artifact={artifact} versions={guestVersions} />
          </div>
        )}
      </div>
      <footer className="drawer-footer">
        <p className="share-save-status" aria-live="polite">
          {changed
            ? translate("변경한 공유 범위는 저장 후 적용됩니다.")
            : translate("링크를 복사해도 공유 범위는 바뀌지 않습니다.")}
        </p>
        <button className="button secondary" onClick={onClose}>
          {translate("취소")}
        </button>
        <button
          className="button primary"
          disabled={
            !canManage || a.busy || (visibility === "selected" && !ids.length)
          }
          onClick={() =>
            void a.run(async () => {
              await api(
                `/api/artifacts/${artifact.id}/sharing`,
                json("PATCH", {
                  visibility,
                  memberIds: visibility === "selected" ? ids : [],
                  expiresAt: expires ? new Date(expires).toISOString() : null,
                }),
              );
              onSaved();
              onClose();
            })
          }
        >
          {a.busy ? <Spinner /> : translate("공유 설정 저장")}
        </button>
      </footer>
    </Modal>
  );
}

export function UploadDialog({
  onClose,
  onSaved,
  existing,
}: {
  onClose: () => void;
  onSaved: () => void;
  existing?: Artifact;
}) {
  useLocale();
  const [files, setFiles] = useState<File[]>([]);
  const [title, setTitle] = useState(existing?.title || "");
  const [description, setDescription] = useState(existing?.description || "");
  const [message, setMessage] = useState("");
  const [draft, setDraft] = useState(existing?.reviewEnabled || false);
  const a = useAction();
  function choose(input: FileList | null) {
    if (input) {
      const selected = Array.from(input);
      setFiles(selected);
      if (!title && selected.length)
        setTitle(
          (
            selected[0].webkitRelativePath?.split("/")[0] || selected[0].name
          ).replace(/\.[^.]+$/, ""),
        );
    }
  }
  return (
    <Modal
      title={existing ? translate("새 버전 게시") : translate("새 아티팩트")}
      wide
      onClose={onClose}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void a.run(async () => {
            const folder = files.some((f) => f.webkitRelativePath);
            const paths = files.map((f) =>
              folder
                ? (f.webkitRelativePath || f.name)
                    .split("/")
                    .slice(1)
                    .join("/") || f.name
                : f.name,
            );
            if (!paths.includes("index.html"))
              throw new Error(
                translate(
                  "시작 파일 index.html이 필요합니다. HTML 이름을 index.html로 바꾸거나 정적 웹앱 폴더를 선택해 주세요.",
                ),
              );
            const metadata = {
              title,
              description,
              message,
              draft,
              baseVersionId: existing?.currentVersionId,
              files: files.map((f, i) => ({
                path: paths[i],
                mimeType: f.type,
              })),
              visibility: existing?.visibility || "private",
              memberIds: existing?.memberIds || [],
            };
            const body = new FormData();
            body.append("metadata", JSON.stringify(metadata));
            files.forEach((f, i) => body.append(`file${i}`, f, f.name));
            await api(
              existing
                ? `/api/artifacts/${existing.id}/versions`
                : "/api/artifacts",
              { method: "POST", body },
            );
            onSaved();
            onClose();
          });
        }}
      >
        <div className="modal-body">
          <label className="draft-option">
            <input
              type="checkbox"
              checked={draft}
              disabled={!!existing?.reviewEnabled}
              onChange={(e) => setDraft(e.target.checked)}
            />
            <span>
              <b>{translate("초안으로 저장")}</b>
              <small>
                {translate("공식 링크는 승인한 버전을 유지합니다.")}
              </small>
            </span>
          </label>
          <div
            className="upload-drop"
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              choose(e.dataTransfer.files);
            }}
          >
            <UploadCloud size={32} />
            <b>
              {files.length
                ? translate("{p0}개 파일 준비됨", { p0: files.length })
                : translate("완성한 작업을 여기에 놓으세요")}
            </b>
            <p>
              {translate(
                "index.html과 필요한 파일 또는 정적 웹앱 폴더를 올려주세요.",
              )}
            </p>
            <div className="button-row">
              <label className="button secondary">
                {translate("파일 선택")}
                <input
                  type="file"
                  multiple
                  onChange={(e) => choose(e.target.files)}
                  hidden
                />
              </label>
              <label className="button secondary">
                {translate("폴더 선택")}
                <input
                  type="file"
                  multiple
                  {...{ webkitdirectory: "" }}
                  onChange={(e) => choose(e.target.files)}
                  hidden
                />
              </label>
            </div>
          </div>
          {files.length > 0 && (
            <div className="upload-files">
              <b>{bytes(files.reduce((n, f) => n + f.size, 0))}</b>
              <span>
                {files
                  .slice(0, 4)
                  .map((f) => f.webkitRelativePath || f.name)
                  .join(", ")}
                {files.length > 4 ? "…" : ""}
              </span>
            </div>
          )}
          <Field label={translate("제목")}>
            <input
              required
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={200}
              placeholder={translate("어떤 작업인가요?")}
            />
          </Field>
          <Field label={translate("설명")}>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              placeholder={translate("팀원에게 보여줄 짧은 설명")}
            />
          </Field>
          {existing && (
            <Field label={translate("버전 메모")}>
              <input
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                placeholder={translate("이번 버전에서 달라진 점")}
              />
            </Field>
          )}
          <Notice>
            {existing
              ? translate("기존 공유 설정과 이전 버전을 유지합니다.")
              : translate("처음 올린 아티팩트는 나만 볼 수 있습니다.")}
          </Notice>
          <ErrorBox error={a.error} />
        </div>
        <footer className="dialog-footer">
          <button type="button" className="button secondary" onClick={onClose}>
            {translate("취소")}
          </button>
          <button className="button primary" disabled={!files.length || a.busy}>
            {a.busy ? <Spinner /> : <Plus size={17} />}
            {existing ? translate("새 버전 게시") : translate("아티팩트 게시")}
          </button>
        </footer>
      </form>
    </Modal>
  );
}

export function DetailPanel({
  artifact,
  onClose,
  onShare,
  onChanged,
  onUpload,
  canManage = true,
  user,
}: {
  artifact: Artifact;
  user: User;
  onClose: () => void;
  onShare: () => void;
  onChanged: () => void;
  onUpload: () => void;
  canManage?: boolean;
}) {
  useLocale();
  const [versions, setVersions] = useState<Version[]>([]);
  const [tab, setTab] = useState("preview");
  const [versionId, setVersionId] = useState(artifact.currentVersionId);
  useEffect(
    () => setVersionId(artifact.currentVersionId),
    [artifact.id, artifact.currentVersionId],
  );
  const chosenVersion =
    versions.find((v) => v.id === versionId) ||
    versions.find((v) => v.id === artifact.currentVersionId);
  const previewArtifact = chosenVersion
    ? { ...artifact, currentVersionId: chosenVersion.id }
    : artifact;
  const [confirm, setConfirm] = useState("");
  const a = useAction();
  useEffect(() => {
    api<{ versions: Version[] }>(`/api/artifacts/${artifact.id}`)
      .then((r) => setVersions(r.versions || []))
      .catch(a.setError);
  }, [artifact.id, artifact.currentVersionId]);
  return (
    <Modal
      title={artifact.title}
      drawer
      wide
      className="artifact-detail"
      onClose={onClose}
    >
      <div className="drawer-body detail-body">
        <p className="detail-description">
          {artifact.description || translate("아직 설명이 없습니다.")}
        </p>
        <div className="detail-actions">
          <a
            className="button secondary compact"
            href={artifact.shareUrl}
            target="_blank"
            rel="noreferrer"
          >
            <ArrowUpRight size={16} />
            {translate("새 창")}
          </a>
          <button className="button primary compact" onClick={onShare}>
            <Share2 size={16} />
            {translate("공유")}
          </button>
        </div>
        <div className="detail-layout">
          <section className="detail-reading">
            {versions.length > 0 && (
              <div className="detail-version-picker">
                <label htmlFor="detail-version">
                  {translate("검토할 버전")}
                </label>
                <Select
                  id="detail-version"
                  value={chosenVersion?.id || versionId}
                  onValueChange={(value) => setVersionId(value)}
                >
                  {versions
                    .slice()
                    .reverse()
                    .map((v) => (
                      <option key={v.id} value={v.id}>
                        v{v.number} ·{" "}
                        {v.id === artifact.publishedVersionId
                          ? translate("공식 버전")
                          : v.published
                            ? translate("게시됨")
                            : translate("초안")}
                      </option>
                    ))}
                </Select>
                {chosenVersion && (
                  <CopyButton
                    text={`${artifact.shareUrl}?version=${encodeURIComponent(chosenVersion.id)}`}
                    label={translate("버전 링크")}
                  />
                )}
              </div>
            )}
            <div className="tabs">
              <button
                className={tab === "preview" ? "active" : ""}
                onClick={() => setTab("preview")}
              >
                <BookOpen size={15} />
                {translate("미리보기")}
              </button>
              <button
                className={tab === "versions" ? "active" : ""}
                onClick={() => setTab("versions")}
              >
                <History size={15} />
                {translate("버전 {p0}", { p0: versions.length })}
              </button>
              <button
                className={tab === "review" ? "active" : ""}
                onClick={() => setTab("review")}
              >
                {translate("검토 및 피드백")}
              </button>
              <button
                className={tab === "compare" ? "active" : ""}
                onClick={() => setTab("compare")}
              >
                {translate("버전 비교")}
              </button>
              <button
                className={tab === "context" ? "active" : ""}
                onClick={() => setTab("context")}
              >
                {translate("출처")}
              </button>
            </div>
            {tab === "preview" ? (
              <ArtifactPreview artifact={previewArtifact} className="large" />
            ) : tab === "review" ? (
              chosenVersion && (
                <ReviewPanel
                  artifact={artifact}
                  version={chosenVersion}
                  user={user}
                  onChange={onChanged}
                />
              )
            ) : tab === "compare" ? (
              <VersionCompare artifact={artifact} versions={versions} />
            ) : tab === "context" ? (
              <ArtifactContext
                artifact={artifact}
                version={chosenVersion}
                user={user}
                onChange={onChanged}
              />
            ) : (
              <div className="version-list">
                {versions.map((v) => (
                  <div className="version-item" key={v.id}>
                    <span className="version-badge">v{v.number}</span>
                    <div>
                      <b>{v.message || translate("아티팩트 게시")}</b>
                      <small>
                        {date(v.createdAt)} · {bytes(v.sizeBytes)} ·{" "}
                        {translate("{p0}개 파일", { p0: v.files.length })}
                      </small>
                    </div>
                    {v.id === artifact.currentVersionId ? (
                      <span className="badge">{translate("현재")}</span>
                    ) : (
                      <button
                        className="button secondary compact"
                        disabled={!canManage || a.busy}
                        onClick={() => setConfirm(`version:${v.id}`)}
                      >
                        <RotateCcw size={14} />
                        {translate("복원")}
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </section>
          <aside
            className="detail-inspector"
            aria-label={translate("아티팩트 정보")}
          >
            <h3>{translate("아티팩트 정보")}</h3>
            <dl className="detail-list">
              <div>
                <dt>{translate("작성자")}</dt>
                <dd>
                  <Avatar
                    name={artifact.ownerName || "F"}
                    src={artifact.ownerAvatarUrl}
                  />
                  {artifact.ownerName}
                </dd>
              </div>
              <div>
                <dt>{translate("공유 범위")}</dt>
                <dd className={`scope-label scope-${artifact.visibility}`}>
                  <VisibilityIcon value={artifact.visibility} />
                  {visibilityLabels[artifact.visibility]}
                </dd>
              </div>
              <div>
                <dt>{translate("파일 크기")}</dt>
                <dd>{bytes(artifact.sizeBytes)}</dd>
              </div>
              <div>
                <dt>{translate("마지막 게시")}</dt>
                <dd>{date(artifact.updatedAt)}</dd>
              </div>
              {artifact.expiresAt && (
                <div>
                  <dt>{translate("공유 만료")}</dt>
                  <dd>{date(artifact.expiresAt)}</dd>
                </div>
              )}
              <div>
                <dt>{translate("게시한 버전")}</dt>
                <dd>{translate("{p0}개", { p0: versions.length })}</dd>
              </div>
            </dl>
            <button className="button secondary" onClick={onShare}>
              <Share2 size={16} />
              {translate("공유 설정 보기")}
            </button>
          </aside>
        </div>
        <ErrorBox error={a.error} />
        {confirm && (
          <div className="confirm-inline" role="alert">
            <p>
              {confirm === "delete"
                ? translate(
                    "이 아티팩트를 휴지통으로 옮길까요? 30일 동안 복원할 수 있습니다.",
                  )
                : translate(
                    "이 버전을 현재 버전으로 복원할까요? 이전 버전은 보존됩니다.",
                  )}
            </p>
            <div className="button-row">
              <button
                className="button secondary compact"
                onClick={() => setConfirm("")}
              >
                {translate("취소")}
              </button>
              <button
                className={`button compact ${confirm === "delete" ? "danger" : "primary"}`}
                disabled={a.busy}
                onClick={() =>
                  void a.run(async () => {
                    if (confirm === "delete")
                      await api(
                        `/api/artifacts/${artifact.id}`,
                        json("DELETE"),
                      );
                    else
                      await api(
                        `/api/artifacts/${artifact.id}/versions/${confirm.slice(8)}/restore`,
                        json("POST"),
                      );
                    setConfirm("");
                    onChanged();
                    if (confirm === "delete") onClose();
                  })
                }
              >
                {translate("확인")}
              </button>
            </div>
          </div>
        )}
      </div>
      <footer className="drawer-footer detail-footer">
        <button
          className="button secondary compact"
          disabled={!canManage}
          onClick={onUpload}
        >
          <UploadCloud size={15} />
          {translate("새 버전")}
        </button>
        <a
          className="button secondary compact"
          href={`/api/artifacts/${artifact.id}/download`}
        >
          <Download size={15} />
          {translate("다운로드")}
        </a>
        <button
          className="icon-button"
          title={
            artifact.archived
              ? translate("아카이브에서 복원")
              : translate("아카이브")
          }
          aria-label={
            artifact.archived
              ? translate("아카이브에서 복원")
              : translate("아카이브")
          }
          disabled={!canManage || a.busy}
          onClick={() =>
            void a.run(async () => {
              await api(
                `/api/artifacts/${artifact.id}/${artifact.archived ? "restore" : "archive"}`,
                json("POST"),
              );
              onChanged();
              onClose();
            })
          }
        >
          {artifact.archived ? <RotateCcw size={17} /> : <Archive size={17} />}
        </button>
        <button
          className="icon-button danger-text"
          aria-label={translate("휴지통으로 이동")}
          disabled={!canManage}
          onClick={() => setConfirm("delete")}
        >
          <Trash2 size={17} />
        </button>
      </footer>
    </Modal>
  );
}

export function LibraryEmpty({
  view,
  onUpload,
}: {
  view: string;
  onUpload: () => void;
}) {
  useLocale();
  return (
    <Empty
      icon={
        view === "archive" ? (
          <Archive size={34} />
        ) : view === "shared" ? (
          <Users size={34} />
        ) : (
          <FileCode size={34} />
        )
      }
      heading={
        view === "archive"
          ? translate("보관한 작업이 여기에 모입니다")
          : view === "shared"
            ? translate("함께 볼 작업을 기다리고 있어요")
            : translate("첫 번째 아티팩트를 올려보세요")
      }
      description={
        view === "archive"
          ? translate(
              "아카이브한 아티팩트는 자동 정리에서 제외하고 안전하게 보관합니다.",
            )
          : view === "shared"
            ? translate(
                "조직 전체 또는 나에게 공유된 아티팩트를 여기에서 볼 수 있습니다.",
              )
            : translate(
                "보고서, 프로토타입, 작은 웹앱. 만든 것을 올리거나 에이전트에게 게시를 맡기세요.",
              )
      }
      action={
        view !== "archive" && view !== "shared" ? (
          <button className="button primary" onClick={onUpload}>
            <Plus size={17} />
            {translate("새 아티팩트")}
          </button>
        ) : undefined
      }
    />
  );
}
