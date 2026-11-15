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
  private: "나만 보기",
  organization: "조직 공유",
  selected: "선택한 사람",
  public: "전체 공개",
};
export function VisibilityIcon({
  value,
  size = 14,
}: {
  value: Visibility;
  size?: number;
}) {
  const Icon = value === "private" ? Lock : value === "public" ? Globe : Users;
  return <Icon size={size} />;
}
export function ArtifactPreview({
  artifact,
  className = "",
}: {
  artifact: Artifact;
  className?: string;
}) {
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
        json("POST", {}),
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
  }, [artifact.id, artifact.currentVersionId]);
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
          title={`${artifact.title} 미리보기`}
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
              ? "미리보기를 불러오지 못했습니다."
              : artifact.kind?.toUpperCase() || "ARTIFACT"}
          </span>
        </div>
      )}
    </div>
  );
}
export function ArtifactCard({
  artifact,
  onOpen,
  onShare,
}: {
  artifact: Artifact;
  onOpen: () => void;
  onShare: () => void;
}) {
  return (
    <article className="artifact-card">
      <button
        className="artifact-open"
        onClick={onOpen}
        aria-label={`${artifact.title} 열기`}
      >
        <ArtifactPreview artifact={artifact} />
        <h3>{artifact.title}</h3>
        <p className="artifact-description">
          {artifact.description || "설명 없음"}
        </p>
      </button>
      <div className="artifact-meta">
        <span className="owner-name">
          <Avatar name={artifact.ownerName || "F"} />
          <span>
            <span>{artifact.ownerName || "—"}</span>
            <small>{relative(artifact.updatedAt)} 수정</small>
          </span>
        </span>
        <span className={`scope-label scope-${artifact.visibility}`}>
          <VisibilityIcon value={artifact.visibility} />
          {visibilityLabels[artifact.visibility]}
        </span>
        <button
          className="icon-button artifact-share"
          onClick={onShare}
          aria-label={`${artifact.title} 공유`}
          title="공유 설정"
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
      title="아티팩트 공유"
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
              {artifact.ownerName} · {relative(artifact.updatedAt)} 수정
            </small>
          </div>
        </div>
        <div className="share-current-access">
          <span>현재 공유 범위</span>
          <span className={`scope-label scope-${artifact.visibility}`}>
            <VisibilityIcon value={artifact.visibility} />
            {visibilityLabels[artifact.visibility]}
          </span>
        </div>
        <fieldset className="visibility-options">
          <legend>공유 범위</legend>
          {(
            [
              {
                value: "private",
                label: "나만 보기",
                description: "작성자와 관리자만 접근",
              },
              {
                value: "organization",
                label: "조직 구성원",
                description: "로그인한 모든 조직 구성원",
              },
              {
                value: "selected",
                label: "선택한 구성원",
                description: "지정한 구성원에게만 공유",
              },
              {
                value: "public",
                label: "링크가 있는 모든 사람",
                description: "로그인 없이 누구나 열람",
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
              <span>함께 볼 사람 선택</span>
              <div className="input-icon">
                <Search size={17} />
                <input
                  value={q}
                  onChange={(e) => setQ(e.target.value)}
                  placeholder="이름 또는 이메일 검색"
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
                      aria-label={`${u.name} 선택`}
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
                    <Avatar name={u.name} />
                    <span>
                      <b>{u.name}</b>
                      <small>{u.email}</small>
                    </span>
                    <span className="person-role">열람</span>
                  </label>
                ))}
              {!members.length && (
                <p className="subtle">아직 선택할 구성원이 없습니다.</p>
              )}
            </div>
          </section>
        )}
        <div className="access-note">
          <VisibilityIcon value={visibility} size={18} />
          <p>
            {visibility === "private"
              ? "작성자와 관리자만 열 수 있습니다."
              : visibility === "organization"
                ? "로그인한 조직 구성원이 열 수 있습니다."
                : visibility === "selected"
                  ? "선택한 구성원과 작성자·관리자만 열 수 있습니다."
                  : "링크를 전달받은 누구나 로그인 없이 열 수 있습니다."}
          </p>
        </div>
        <Field
          label="공유 만료"
          hint="만료 시간이 지나면 작성자와 관리자만 볼 수 있습니다."
        >
          <input
            disabled={!canManage}
            aria-label="공유 만료"
            type="datetime-local"
            value={expires}
            onChange={(e) => setExpires(e.target.value)}
          />
        </Field>
        <section className="share-link">
          <label>공유 링크</label>
          <div className="link-field">
            <Link size={16} />
            <input
              readOnly
              value={
                artifact.shareUrl || `${location.origin}/s/${artifact.slug}`
              }
              aria-label="공유 링크"
            />
          </div>
          <CopyButton
            text={artifact.shareUrl || `${location.origin}/s/${artifact.slug}`}
            label="링크 복사"
          />
        </section>
        <ErrorBox error={a.error} />
      </div>
      <footer className="drawer-footer">
        <p className="share-save-status" aria-live="polite">
          {changed
            ? "변경한 공유 범위는 저장 후 적용됩니다."
            : "링크를 복사해도 공유 범위는 바뀌지 않습니다."}
        </p>
        <button className="button secondary" onClick={onClose}>
          취소
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
          {a.busy ? <Spinner /> : "공유 설정 저장"}
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
  const [files, setFiles] = useState<File[]>([]);
  const [title, setTitle] = useState(existing?.title || "");
  const [description, setDescription] = useState(existing?.description || "");
  const [message, setMessage] = useState("");
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
      title={existing ? "새 버전 게시" : "새 아티팩트"}
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
                "시작 파일 index.html이 필요합니다. HTML 이름을 index.html로 바꾸거나 정적 웹앱 폴더를 선택해 주세요.",
              );
            const metadata = {
              title,
              description,
              message,
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
                ? `${files.length}개 파일 준비됨`
                : "완성한 작업을 여기에 놓으세요"}
            </b>
            <p>index.html과 필요한 파일 또는 정적 웹앱 폴더를 올려주세요.</p>
            <div className="button-row">
              <label className="button secondary">
                파일 선택
                <input
                  type="file"
                  multiple
                  onChange={(e) => choose(e.target.files)}
                  hidden
                />
              </label>
              <label className="button secondary">
                폴더 선택
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
          <Field label="제목">
            <input
              required
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={200}
              placeholder="어떤 작업인가요?"
            />
          </Field>
          <Field label="설명">
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              placeholder="팀원에게 보여줄 짧은 설명"
            />
          </Field>
          {existing && (
            <Field label="버전 메모">
              <input
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                placeholder="이번 버전에서 달라진 점"
              />
            </Field>
          )}
          <Notice>
            {existing
              ? "기존 공유 설정과 이전 버전을 유지합니다."
              : "처음 올린 아티팩트는 나만 볼 수 있습니다."}
          </Notice>
          <ErrorBox error={a.error} />
        </div>
        <footer className="dialog-footer">
          <button type="button" className="button secondary" onClick={onClose}>
            취소
          </button>
          <button className="button primary" disabled={!files.length || a.busy}>
            {a.busy ? <Spinner /> : <Plus size={17} />}
            {existing ? "새 버전 게시" : "아티팩트 게시"}
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
}: {
  artifact: Artifact;
  onClose: () => void;
  onShare: () => void;
  onChanged: () => void;
  onUpload: () => void;
  canManage?: boolean;
}) {
  const [versions, setVersions] = useState<Version[]>([]);
  const [tab, setTab] = useState("preview");
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
          {artifact.description || "아직 설명이 없습니다."}
        </p>
        <div className="detail-actions">
          <a
            className="button secondary compact"
            href={artifact.shareUrl}
            target="_blank"
            rel="noreferrer"
          >
            <ArrowUpRight size={16} />새 창
          </a>
          <button className="button primary compact" onClick={onShare}>
            <Share2 size={16} />
            공유
          </button>
        </div>
        <div className="detail-layout">
          <section className="detail-reading">
            <div className="tabs">
              <button
                className={tab === "preview" ? "active" : ""}
                onClick={() => setTab("preview")}
              >
                <BookOpen size={15} />
                미리보기
              </button>
              <button
                className={tab === "versions" ? "active" : ""}
                onClick={() => setTab("versions")}
              >
                <History size={15} />
                버전 {versions.length}
              </button>
            </div>
            {tab === "preview" ? (
              <ArtifactPreview artifact={artifact} className="large" />
            ) : (
              <div className="version-list">
                {versions.map((v) => (
                  <div className="version-item" key={v.id}>
                    <span className="version-badge">v{v.number}</span>
                    <div>
                      <b>{v.message || "아티팩트 게시"}</b>
                      <small>
                        {date(v.createdAt)} · {bytes(v.sizeBytes)} ·{" "}
                        {v.files.length}개 파일
                      </small>
                    </div>
                    {v.id === artifact.currentVersionId ? (
                      <span className="badge">현재</span>
                    ) : (
                      <button
                        className="button secondary compact"
                        disabled={!canManage || a.busy}
                        onClick={() => setConfirm(`version:${v.id}`)}
                      >
                        <RotateCcw size={14} />
                        복원
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </section>
          <aside className="detail-inspector" aria-label="아티팩트 정보">
            <h3>아티팩트 정보</h3>
            <dl className="detail-list">
              <div>
                <dt>작성자</dt>
                <dd>
                  <Avatar name={artifact.ownerName || "F"} />
                  {artifact.ownerName}
                </dd>
              </div>
              <div>
                <dt>공유 범위</dt>
                <dd className={`scope-label scope-${artifact.visibility}`}>
                  <VisibilityIcon value={artifact.visibility} />
                  {visibilityLabels[artifact.visibility]}
                </dd>
              </div>
              <div>
                <dt>파일 크기</dt>
                <dd>{bytes(artifact.sizeBytes)}</dd>
              </div>
              <div>
                <dt>마지막 게시</dt>
                <dd>{date(artifact.updatedAt)}</dd>
              </div>
              {artifact.expiresAt && (
                <div>
                  <dt>공유 만료</dt>
                  <dd>{date(artifact.expiresAt)}</dd>
                </div>
              )}
              <div>
                <dt>게시한 버전</dt>
                <dd>{versions.length}개</dd>
              </div>
            </dl>
            <button className="button secondary" onClick={onShare}>
              <Share2 size={16} />
              공유 설정 보기
            </button>
          </aside>
        </div>
        <ErrorBox error={a.error} />
        {confirm && (
          <div className="confirm-inline" role="alert">
            <p>
              {confirm === "delete"
                ? "이 아티팩트와 모든 버전을 영구 삭제할까요?"
                : "이 버전을 현재 버전으로 복원할까요? 이전 버전은 보존됩니다."}
            </p>
            <div className="button-row">
              <button
                className="button secondary compact"
                onClick={() => setConfirm("")}
              >
                취소
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
                확인
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
          <UploadCloud size={15} />새 버전
        </button>
        <a
          className="button secondary compact"
          href={`/api/artifacts/${artifact.id}/download`}
        >
          <Download size={15} />
          다운로드
        </a>
        <button
          className="icon-button"
          title={artifact.archived ? "아카이브에서 복원" : "아카이브"}
          aria-label={artifact.archived ? "아카이브에서 복원" : "아카이브"}
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
          aria-label="영구 삭제"
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
          ? "보관한 작업이 여기에 모입니다"
          : view === "shared"
            ? "함께 볼 작업을 기다리고 있어요"
            : "첫 번째 아티팩트를 올려보세요"
      }
      description={
        view === "archive"
          ? "아카이브한 아티팩트는 자동 정리에서 제외하고 안전하게 보관합니다."
          : view === "shared"
            ? "조직 전체 또는 나에게 공유된 아티팩트를 여기에서 볼 수 있습니다."
            : "보고서, 프로토타입, 작은 웹앱. 만든 것을 올리거나 에이전트에게 게시를 맡기세요."
      }
      action={
        view !== "archive" && view !== "shared" ? (
          <button className="button primary" onClick={onUpload}>
            <Plus size={17} />새 아티팩트
          </button>
        ) : undefined
      }
    />
  );
}
