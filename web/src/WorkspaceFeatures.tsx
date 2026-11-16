import { useCallback, useEffect, useId, useRef, useState } from "react";
import {
  ArrowDown,
  ArrowUp,
  Check,
  Folder,
  Plus,
  Share2,
  Star,
  Trash2,
  X,
} from "lucide-react";
import { api, bytes, date, json } from "./api";
import { ArtifactPreview, VisibilityIcon, visibilityLabels } from "./Artifacts";
import { CopyButton, ErrorBox, Field, Spinner } from "./ui";
import { number, t, useLocale } from "./i18n";
import type { Artifact, User, Version } from "./types";
import "./WorkspaceFeatures.css";

interface Collection {
  id: string;
  name: string;
  description: string;
  ownerId: string;
  visibility: "private" | "organization";
  artifactIds: string[];
  createdAt: string;
  updatedAt: string;
}
interface Provenance {
  agent?: string;
  model?: string;
  summary?: string;
  repository?: string;
  commit?: string;
  taskUrl?: string;
  sources?: string[];
}
type ContextArtifact = Artifact & {
  tags?: string[];
  favorite?: boolean;
  provenance?: Provenance;
};
type ContextVersion = Version & { provenance?: Provenance };
interface CollectionDraft {
  id?: string;
  name: string;
  description: string;
  visibility: "private" | "organization";
  artifactIds: string[];
  baseUpdatedAt?: string;
}
export function CollectionsView({
  user,
  onOpen,
  onShare,
}: {
  user: User;
  onOpen: (artifact: Artifact) => void;
  onShare?: (artifact: Artifact) => void;
}) {
  useLocale();
  const [collections, setCollections] = useState<Collection[]>([]);
  const [available, setAvailable] = useState<Artifact[]>([]);
  const [selected, setSelected] = useState<Collection | null>(null);
  const [items, setItems] = useState<Artifact[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [draft, setDraft] = useState<CollectionDraft | null>(null);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [deleting, setDeleting] = useState(false);
  const [notice, setNotice] = useState("");
  const collectionHeading = useId();
  const refresh = useCallback(async () => {
    const [c, a] = await Promise.all([
      api<{ collections: Collection[] }>("/api/collections"),
      api<{ artifacts: Artifact[] }>("/api/artifacts"),
    ]);
    setCollections(c.collections);
    setAvailable(a.artifacts);
    return c.collections;
  }, []);
  useEffect(() => {
    let active = true;
    void refresh()
      .then((list) => {
        if (active) setSelectedId((old) => old || list[0]?.id || "");
      })
      .catch((e: unknown) => {
        if (active) setError(e);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [refresh]);
  useEffect(() => {
    const controller = new AbortController();
    setDeleting(false);
    setSelected(null);
    setItems([]);
    if (!selectedId) {
      setDetailLoading(false);
      return () => controller.abort();
    }
    setDetailLoading(true);
    void api<{ collection: Collection; artifacts: Artifact[] }>(
      `/api/collections/${encodeURIComponent(selectedId)}`,
      { signal: controller.signal },
    )
      .then((out) => {
        if (!controller.signal.aborted) {
          setSelected(out.collection);
          setItems(out.artifacts);
        }
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setError(e);
      })
      .finally(() => {
        if (!controller.signal.aborted) setDetailLoading(false);
      });
    return () => controller.abort();
  }, [selectedId, collections]);
  const manager =
    !!selected && (selected.ownerId === user.id || user.role === "admin");
  const known = new Map([...available, ...items].map((a) => [a.id, a]));
  const choices = [...known.values()].filter(
    (a) =>
      !draft?.artifactIds.includes(a.id) &&
      `${a.title} ${a.description}`
        .toLocaleLowerCase()
        .includes(search.toLocaleLowerCase()),
  );
  function beginCreate() {
    setError(null);
    setNotice("");
    setSearch("");
    setDraft({
      name: "",
      description: "",
      visibility: "private",
      artifactIds: [],
    });
  }
  function beginEdit() {
    if (selected) {
      setError(null);
      setNotice("");
      setSearch("");
      setDraft({
        ...selected,
        baseUpdatedAt: selected.updatedAt,
        artifactIds: [...selected.artifactIds],
      });
    }
  }
  function move(index: number, delta: number) {
    if (
      !draft ||
      index + delta < 0 ||
      index + delta >= draft.artifactIds.length
    )
      return;
    const ids = [...draft.artifactIds];
    [ids[index], ids[index + delta]] = [ids[index + delta], ids[index]];
    setDraft({ ...draft, artifactIds: ids });
  }
  async function save() {
    if (!draft) return;
    setBusy(true);
    setError(null);
    setNotice("");
    try {
      const out = await api<{ collection: Collection }>(
        `/api/collections${draft.id ? `/${encodeURIComponent(draft.id)}` : ""}`,
        json(draft.id ? "PATCH" : "POST", {
          name: draft.name,
          description: draft.description,
          visibility: draft.visibility,
          artifactIds: draft.artifactIds,
          baseUpdatedAt: draft.baseUpdatedAt,
        }),
      );
      setDraft(null);
      await refresh();
      setSelectedId(out.collection.id);
      setNotice(t("컬렉션을 저장했습니다."));
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    if (!selected) return;
    setBusy(true);
    setError(null);
    try {
      await api(
        `/api/collections/${encodeURIComponent(selected.id)}`,
        json("DELETE"),
      );
      setSelectedId("");
      setSelected(null);
      setDeleting(false);
      const list = await refresh();
      setSelectedId(list[0]?.id || "");
      setNotice(t("컬렉션을 삭제했습니다."));
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="collections-view" aria-labelledby={collectionHeading}>
      <header className="workspace-feature-heading">
        <div>
          <h1 id={collectionHeading}>{t("컬렉션")}</h1>
          <p>{t("아티팩트를 목적에 맞게 모으고 순서를 정리하세요.")}</p>
        </div>
        <button
          className="button primary"
          disabled={busy}
          onClick={beginCreate}
        >
          <Plus size={17} />
          {t("새 컬렉션")}
        </button>
      </header>
      <ErrorBox error={error} />
      {notice && (
        <p className="workspace-feature-notice" role="status">
          <Check size={16} />
          {t(notice)}
        </p>
      )}
      {loading ? (
        <div className="workspace-feature-loading">
          <Spinner />
          {t("컬렉션을 불러오는 중")}
        </div>
      ) : (
        <div className="collections-layout">
          <nav className="collections-nav" aria-label={t("컬렉션 목록")}>
            {!collections.length && (
              <p className="workspace-feature-muted">
                {t("아직 컬렉션이 없습니다.")}
              </p>
            )}
            {collections.map((c) => (
              <button
                key={c.id}
                className={selectedId === c.id && !draft ? "is-selected" : ""}
                aria-current={
                  selectedId === c.id && !draft ? "page" : undefined
                }
                disabled={busy}
                onClick={() => {
                  setSelectedId(c.id);
                  setDraft(null);
                  setError(null);
                }}
              >
                <Folder size={17} />
                <span>
                  <strong>{c.name}</strong>
                  <small>
                    {c.visibility === "private"
                      ? t("나만 보기")
                      : t("조직 공유")}{" "}
                    · {t("{count}개", { count: c.artifactIds.length })}
                  </small>
                </span>
              </button>
            ))}
          </nav>
          <div className="collections-content">
            {draft ? (
              <form
                className="collection-editor"
                onSubmit={(e) => {
                  e.preventDefault();
                  void save();
                }}
              >
                <header className="workspace-feature-subheading">
                  <h2>{draft.id ? t("컬렉션 편집") : t("새 컬렉션")}</h2>
                  <button
                    type="button"
                    className="button ghost compact"
                    disabled={busy}
                    onClick={() => setDraft(null)}
                  >
                    <X size={16} />
                    {t("취소")}
                  </button>
                </header>
                <Field label={t("이름")}>
                  <input
                    required
                    maxLength={200}
                    value={draft.name}
                    disabled={busy}
                    onChange={(e) =>
                      setDraft({ ...draft, name: e.target.value })
                    }
                    autoFocus
                  />
                </Field>
                <Field label={t("설명")}>
                  <textarea
                    rows={2}
                    maxLength={4000}
                    value={draft.description}
                    disabled={busy}
                    onChange={(e) =>
                      setDraft({ ...draft, description: e.target.value })
                    }
                  />
                </Field>
                <Field
                  label={t("공유 범위")}
                  hint={t(
                    "컬렉션에 추가해도 아티팩트의 접근 권한은 바뀌지 않습니다.",
                  )}
                >
                  <select
                    value={draft.visibility}
                    disabled={busy}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        visibility: e.target.value as
                          "private" | "organization",
                      })
                    }
                  >
                    <option value="private">{t("나만 보기")}</option>
                    <option value="organization">{t("조직 공유")}</option>
                  </select>
                </Field>
                <h3>{t("아티팩트 순서")}</h3>
                {!draft.artifactIds.length && (
                  <p className="workspace-feature-muted">
                    {t("아래에서 아티팩트를 추가하세요.")}
                  </p>
                )}
                <ol className="collection-order">
                  {draft.artifactIds.map((id, index) => (
                    <li key={id}>
                      <span>{known.get(id)?.title || t("아티팩트")}</span>
                      <div>
                        <button
                          type="button"
                          className="icon-button"
                          disabled={busy || index === 0}
                          aria-label={t("{title} 위로 이동", {
                            title: known.get(id)?.title || t("아티팩트"),
                          })}
                          onClick={() => move(index, -1)}
                        >
                          <ArrowUp size={16} />
                        </button>
                        <button
                          type="button"
                          className="icon-button"
                          disabled={
                            busy || index === draft.artifactIds.length - 1
                          }
                          aria-label={t("{title} 아래로 이동", {
                            title: known.get(id)?.title || t("아티팩트"),
                          })}
                          onClick={() => move(index, 1)}
                        >
                          <ArrowDown size={16} />
                        </button>
                        <button
                          type="button"
                          className="icon-button"
                          disabled={busy}
                          aria-label={t("{title} 컬렉션에서 제거", {
                            title: known.get(id)?.title || t("아티팩트"),
                          })}
                          onClick={() =>
                            setDraft({
                              ...draft,
                              artifactIds: draft.artifactIds.filter(
                                (v) => v !== id,
                              ),
                            })
                          }
                        >
                          <X size={16} />
                        </button>
                      </div>
                    </li>
                  ))}
                </ol>
                <Field label={t("추가할 아티팩트 찾기")}>
                  <input
                    type="search"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder={t("제목 또는 설명 검색")}
                  />
                </Field>
                <div className="collection-choices">
                  {choices.map((a) => (
                    <button
                      type="button"
                      key={a.id}
                      disabled={busy || draft.artifactIds.length >= 500}
                      onClick={() =>
                        setDraft({
                          ...draft,
                          artifactIds: [...draft.artifactIds, a.id],
                        })
                      }
                    >
                      <Plus size={15} />
                      <span>{a.title}</span>
                      <small>{visibilityLabels[a.visibility]}</small>
                    </button>
                  ))}
                  {!choices.length && (
                    <p className="workspace-feature-muted">
                      {t("추가할 수 있는 아티팩트가 없습니다.")}
                    </p>
                  )}
                </div>
                <footer className="workspace-feature-footer">
                  <button
                    type="button"
                    className="button secondary"
                    disabled={busy}
                    onClick={() => setDraft(null)}
                  >
                    {t("취소")}
                  </button>
                  <button
                    className="button primary"
                    disabled={busy || !draft.name.trim()}
                  >
                    {busy ? <Spinner /> : t("컬렉션 저장")}
                  </button>
                </footer>
              </form>
            ) : detailLoading ? (
              <div className="workspace-feature-loading">
                <Spinner />
                {t("아티팩트를 불러오는 중")}
              </div>
            ) : selected ? (
              <>
                <header className="collection-detail-heading">
                  <div>
                    <h2>{selected.name}</h2>
                    {selected.description && <p>{selected.description}</p>}
                    <span className="workspace-feature-muted">
                      {selected.visibility === "private"
                        ? t("나만 보기")
                        : t("조직 공유")}{" "}
                      · {t("{count}개", { count: items.length })}
                    </span>
                  </div>
                  {manager && (
                    <div className="workspace-feature-actions">
                      <button
                        className="button secondary compact"
                        disabled={busy}
                        onClick={beginEdit}
                      >
                        {t("편집")}
                      </button>
                      <button
                        className="icon-button"
                        disabled={busy}
                        aria-label={t("컬렉션 삭제")}
                        onClick={() => setDeleting(true)}
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  )}
                </header>
                {deleting && (
                  <div className="collection-delete-confirm">
                    <p>
                      {t("컬렉션을 삭제할까요? 아티팩트는 그대로 유지됩니다.")}
                    </p>
                    <div>
                      <button
                        className="button secondary compact"
                        disabled={busy}
                        onClick={() => setDeleting(false)}
                      >
                        {t("취소")}
                      </button>
                      <button
                        className="button danger compact"
                        disabled={busy}
                        onClick={() => void remove()}
                      >
                        {t("컬렉션 삭제")}
                      </button>
                    </div>
                  </div>
                )}
                <p className="collection-access-note">
                  {t(
                    "접근 권한이 있는 아티팩트만 표시됩니다. 컬렉션 공유는 개별 아티팩트의 권한을 바꾸지 않습니다.",
                  )}
                </p>
                {!items.length && (
                  <div className="workspace-feature-empty">
                    <Folder size={28} />
                    <h3>{t("아티팩트가 없습니다.")}</h3>
                    <p>
                      {t("컬렉션을 편집해 접근 가능한 아티팩트를 추가하세요.")}
                    </p>
                    {manager && (
                      <button className="button secondary" onClick={beginEdit}>
                        {t("아티팩트 추가")}
                      </button>
                    )}
                  </div>
                )}
                <div className="collection-artifacts">
                  {items.map((a) => (
                    <article key={a.id}>
                      <button
                        className="collection-artifact-open"
                        onClick={() => onOpen(a)}
                      >
                        <ArtifactPreview artifact={a} />
                        <span>
                          <strong>{a.title}</strong>
                          <small>{a.description || t("설명 없음")}</small>
                        </span>
                      </button>
                      <div className="collection-artifact-meta">
                        <span className={`scope-label scope-${a.visibility}`}>
                          <VisibilityIcon value={a.visibility} />
                          {visibilityLabels[a.visibility]}
                        </span>
                        <span className="workspace-feature-muted">
                          {a.ownerName}
                        </span>
                        {onShare && (
                          <button
                            className="icon-button"
                            aria-label={t("{title} 공유", { title: a.title })}
                            onClick={() => onShare(a)}
                          >
                            <Share2 size={16} />
                          </button>
                        )}
                      </div>
                    </article>
                  ))}
                </div>
              </>
            ) : (
              <div className="workspace-feature-empty">
                <Folder size={30} />
                <h2>{t("첫 컬렉션을 만들어 보세요.")}</h2>
                <p>
                  {t("개인 작업이나 팀의 결과물을 한곳에 정리할 수 있습니다.")}
                </p>
                <button className="button primary" onClick={beginCreate}>
                  <Plus size={17} />
                  {t("새 컬렉션")}
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </section>
  );
}

export function ArtifactContext({
  artifact,
  version,
  user,
  onChange,
}: {
  artifact: ContextArtifact;
  version?: ContextVersion;
  user: User | null;
  onChange?: () => void;
}) {
  useLocale();
  const manager =
    !!user && (user.id === artifact.ownerId || user.role === "admin");
  const [editing, setEditing] = useState(false);
  const [tags, setTags] = useState((artifact.tags || []).join(", "));
  const [context, setContext] = useState<Provenance>(artifact.provenance || {});
  const [sources, setSources] = useState(
    (artifact.provenance?.sources || []).join("\n"),
  );
  const [favorite, setFavorite] = useState(!!artifact.favorite);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [saved, setSaved] = useState(false);
  const key = artifact.id;
  const current = useRef(key);
  current.current = key;
  useEffect(() => {
    setFavorite(!!artifact.favorite);
    setEditing(false);
    setError(null);
    setSaved(false);
  }, [artifact.id, artifact.favorite]);
  function edit() {
    setTags((artifact.tags || []).join(", "));
    setContext(artifact.provenance || {});
    setSources((artifact.provenance?.sources || []).join("\n"));
    setEditing(true);
    setSaved(false);
  }
  async function mutate(path: string, body: unknown) {
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      const out = await api<{ artifact: ContextArtifact }>(
        `/api/artifacts/${encodeURIComponent(artifact.id)}/${path}`,
        json(path === "favorite" ? "PUT" : "PATCH", body),
      );
      if (current.current !== key) return;
      setFavorite(!!out.artifact.favorite);
      setEditing(false);
      setSaved(true);
      onChange?.();
    } catch (e) {
      if (current.current === key) setError(e);
    } finally {
      if (current.current === key) setBusy(false);
    }
  }
  function update(field: keyof Provenance, value: string) {
    setContext((old) => ({ ...old, [field]: value }));
  }
  return (
    <section className="artifact-context">
      <header className="workspace-feature-subheading">
        <h3>{t("태그와 제작 정보")}</h3>
        <div className="workspace-feature-actions">
          {user && (
            <button
              type="button"
              className="button ghost compact"
              disabled={busy}
              aria-pressed={favorite}
              onClick={() => void mutate("favorite", { favorite: !favorite })}
            >
              <Star size={16} fill={favorite ? "currentColor" : "none"} />
              {favorite ? t("즐겨찾기 해제") : t("즐겨찾기")}
            </button>
          )}
          {manager && !editing && (
            <button
              className="button secondary compact"
              disabled={busy}
              onClick={edit}
            >
              {t("편집")}
            </button>
          )}
        </div>
      </header>
      <ErrorBox error={error} />
      {saved && (
        <p className="workspace-feature-notice" role="status">
          <Check size={15} />
          {t("변경 사항을 저장했습니다.")}
        </p>
      )}
      {editing ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void mutate("metadata", {
              tags: tags
                .split(",")
                .map((v) => v.trim())
                .filter(Boolean),
              provenance: {
                ...context,
                sources: sources
                  .split("\n")
                  .map((v) => v.trim())
                  .filter(Boolean),
              },
            });
          }}
        >
          <Field
            label={t("태그")}
            hint={t("쉼표로 구분해 최대 30개까지 입력하세요.")}
          >
            <input
              value={tags}
              disabled={busy}
              onChange={(e) => setTags(e.target.value)}
            />
          </Field>
          <div className="context-form-pair">
            <Field label={t("에이전트 이름")}>
              <input
                value={context.agent || ""}
                maxLength={200}
                disabled={busy}
                onChange={(e) => update("agent", e.target.value)}
              />
            </Field>
            <Field label={t("모델")}>
              <input
                value={context.model || ""}
                maxLength={200}
                disabled={busy}
                onChange={(e) => update("model", e.target.value)}
              />
            </Field>
          </div>
          <Field
            label={t("요청 요약")}
            hint={t(
              "비밀번호, 토큰, 전체 대화 등 민감한 정보는 입력하지 마세요.",
            )}
          >
            <textarea
              rows={3}
              maxLength={8000}
              value={context.summary || ""}
              disabled={busy}
              onChange={(e) => update("summary", e.target.value)}
            />
          </Field>
          <Field label={t("저장소 URL")}>
            <input
              type="url"
              value={context.repository || ""}
              maxLength={2048}
              disabled={busy}
              onChange={(e) => update("repository", e.target.value)}
            />
          </Field>
          <div className="context-form-pair">
            <Field label={t("커밋")}>
              <input
                value={context.commit || ""}
                maxLength={200}
                disabled={busy}
                onChange={(e) => update("commit", e.target.value)}
              />
            </Field>
            <Field label={t("작업 URL")}>
              <input
                type="url"
                value={context.taskUrl || ""}
                maxLength={2048}
                disabled={busy}
                onChange={(e) => update("taskUrl", e.target.value)}
              />
            </Field>
          </div>
          <Field
            label={t("출처 URL")}
            hint={t("한 줄에 하나씩, 최대 20개까지 입력하세요.")}
          >
            <textarea
              rows={3}
              value={sources}
              disabled={busy}
              onChange={(e) => setSources(e.target.value)}
            />
          </Field>
          <footer className="workspace-feature-footer">
            <button
              type="button"
              className="button secondary"
              disabled={busy}
              onClick={() => setEditing(false)}
            >
              {t("취소")}
            </button>
            <button className="button primary" disabled={busy}>
              {busy ? <Spinner /> : t("제작 정보 저장")}
            </button>
          </footer>
        </form>
      ) : (
        <>
          <div className="artifact-tags">
            {(artifact.tags || []).map((tag) => (
              <span key={tag}>{tag}</span>
            ))}
            {!artifact.tags?.length && (
              <p className="workspace-feature-muted">
                {t("등록된 태그가 없습니다.")}
              </p>
            )}
          </div>
          <h4>{t("현재 제작 정보")}</h4>
          <ProvenanceView value={artifact.provenance} />
        </>
      )}
      {version && (
        <details className="context-snapshot">
          <summary>
            {t("버전 {number}의 제작 정보", { number: version.number })}
          </summary>
          <p className="workspace-feature-muted">
            {t(
              "업로드 당시의 기록입니다. 현재 제작 정보를 수정해도 이 기록은 바뀌지 않습니다.",
            )}
          </p>
          <ProvenanceView value={version.provenance} />
        </details>
      )}
    </section>
  );
}
function sourceHref(value: string) {
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) &&
      !url.username &&
      !url.password
      ? url.href
      : undefined;
  } catch {
    return undefined;
  }
}
function ProvenanceView({ value }: { value?: Provenance }) {
  const p = value || {};
  if (
    !p.agent &&
    !p.model &&
    !p.summary &&
    !p.repository &&
    !p.commit &&
    !p.taskUrl &&
    !p.sources?.length
  )
    return (
      <p className="workspace-feature-muted">
        {t("등록된 제작 정보가 없습니다.")}
      </p>
    );
  return (
    <dl className="provenance-list">
      {p.agent && (
        <>
          <dt>{t("에이전트")}</dt>
          <dd>{p.agent}</dd>
        </>
      )}
      {p.model && (
        <>
          <dt>{t("모델")}</dt>
          <dd>{p.model}</dd>
        </>
      )}
      {p.summary && (
        <>
          <dt>{t("요청 요약")}</dt>
          <dd className="context-summary">{p.summary}</dd>
        </>
      )}
      {p.repository && (
        <>
          <dt>{t("저장소")}</dt>
          <dd>
            <a
              href={sourceHref(p.repository)}
              target="_blank"
              rel="noopener noreferrer"
            >
              {p.repository}
            </a>
          </dd>
        </>
      )}
      {p.commit && (
        <>
          <dt>{t("커밋")}</dt>
          <dd>
            <code>{p.commit}</code>
          </dd>
        </>
      )}
      {p.taskUrl && (
        <>
          <dt>{t("작업")}</dt>
          <dd>
            <a
              href={sourceHref(p.taskUrl)}
              target="_blank"
              rel="noopener noreferrer"
            >
              {p.taskUrl}
            </a>
          </dd>
        </>
      )}
      {!!p.sources?.length && (
        <>
          <dt>{t("출처")}</dt>
          <dd>
            <ul>
              {p.sources.map((source, index) => (
                <li key={`${source}:${index}`}>
                  <a
                    href={sourceHref(source)}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {source}
                  </a>
                </li>
              ))}
            </ul>
          </dd>
        </>
      )}
    </dl>
  );
}

interface FileDiff {
  path: string;
  status: "added" | "removed" | "modified" | "unchanged";
  beforeBytes: number;
  afterBytes: number;
  beforeHash?: string;
  afterHash?: string;
  lines?: { kind: "added" | "removed" | "context"; text: string }[];
  textOmitted?: boolean;
}
interface Comparison {
  from: Version;
  to: Version;
  files: FileDiff[];
  summary: Record<FileDiff["status"], number>;
  textLimitBytes: number;
  textLimitLines: number;
  previewHosts: string[];
}
const changeLabel = (status: FileDiff["status"]) =>
  ({
    added: t("추가"),
    removed: t("삭제됨"),
    modified: t("변경"),
    unchanged: t("동일"),
  })[status];
export function VersionCompare({
  artifact,
  versions,
}: {
  artifact: Artifact;
  versions: Version[];
}) {
  useLocale();
  const ordered = [...versions].sort((a, b) => b.number - a.number);
  const [from, setFrom] = useState(ordered[1]?.id || ordered[0]?.id || "");
  const [to, setTo] = useState(ordered[0]?.id || "");
  const [path, setPath] = useState("");
  const [data, setData] = useState<Comparison | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const versionKey = ordered.map((v) => v.id).join(":");
  useEffect(() => {
    setFrom((old) =>
      versions.some((v) => v.id === old)
        ? old
        : ordered[1]?.id || ordered[0]?.id || "",
    );
    setTo((old) =>
      versions.some((v) => v.id === old) ? old : ordered[0]?.id || "",
    );
    setPath("");
  }, [artifact.id, versionKey]);
  useEffect(() => {
    const controller = new AbortController();
    setData(null);
    setError(null);
    if (!from || !to) {
      setLoading(false);
      return () => controller.abort();
    }
    setLoading(true);
    const query = new URLSearchParams({ from, to });
    if (path) query.set("path", path);
    void api<Comparison>(
      `/api/artifacts/${encodeURIComponent(artifact.id)}/diff?${query}`,
      { signal: controller.signal },
    )
      .then((out) => {
        if (controller.signal.aborted) return;
        setData(out);
        if (!path)
          setPath(
            out.files.find((f) => f.status !== "unchanged")?.path ||
              out.files[0]?.path ||
              "",
          );
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setError(e);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [artifact.id, from, to, path]);
  if (versions.length < 2)
    return (
      <div className="workspace-feature-empty">
        <h3>{t("비교할 버전이 더 필요합니다.")}</h3>
        <p>{t("두 번째 버전을 업로드하면 변경 내용을 비교할 수 있습니다.")}</p>
      </div>
    );
  const file = data?.files.find((f) => f.path === path);
  return (
    <section className="version-compare">
      <header className="workspace-feature-subheading">
        <h3>{t("버전 비교")}</h3>
      </header>
      <div className="compare-selectors">
        <Field label={t("이전 버전")}>
          <select
            value={from}
            onChange={(e) => {
              setFrom(e.target.value);
              setPath("");
            }}
          >
            {ordered.map((v) => (
              <option key={v.id} value={v.id}>
                {t("버전 {number}", { number: v.number })} · {date(v.createdAt)}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t("이후 버전")}>
          <select
            value={to}
            onChange={(e) => {
              setTo(e.target.value);
              setPath("");
            }}
          >
            {ordered.map((v) => (
              <option key={v.id} value={v.id}>
                {t("버전 {number}", { number: v.number })} · {date(v.createdAt)}
              </option>
            ))}
          </select>
        </Field>
      </div>
      <ErrorBox error={error} />
      {data?.previewHosts && (
        <details className="compare-domain-help">
          <summary>{t("미리보기 도메인")}</summary>
          <p>
            {t("버전마다 독립된 주소에서 실행해 파일이 섞이지 않도록 합니다.")}
          </p>
          <p>
            {t(
              "미리보기가 열리지 않으면 관리자에게 아래 콘텐츠 도메인 연결을 요청하세요.",
            )}
          </p>
          {[...new Set(data.previewHosts)].map((host) => (
            <div className="compare-host" key={host}>
              <code>{host}</code>
              <CopyButton text={host} label={t("도메인 복사")} />
            </div>
          ))}
        </details>
      )}
      <div className="compare-previews">
        <div>
          <h4>
            {t("버전 {number}", {
              number: versions.find((v) => v.id === from)?.number || 0,
            })}
          </h4>
          <ArtifactPreview
            key={`${artifact.id}:${from}`}
            artifact={{
              ...artifact,
              currentVersionId: from,
              title: `${artifact.title} · ${t("버전 {number}", { number: versions.find((v) => v.id === from)?.number || 0 })}`,
            }}
            className="large"
            isolatedVersion
          />
        </div>
        <div>
          <h4>
            {t("버전 {number}", {
              number: versions.find((v) => v.id === to)?.number || 0,
            })}
          </h4>
          <ArtifactPreview
            key={`${artifact.id}:${to}`}
            artifact={{
              ...artifact,
              currentVersionId: to,
              title: `${artifact.title} · ${t("버전 {number}", { number: versions.find((v) => v.id === to)?.number || 0 })}`,
            }}
            className="large"
            isolatedVersion
          />
        </div>
      </div>
      <h4>{t("파일 변경 내용")}</h4>
      {loading && (
        <div className="workspace-feature-loading" role="status">
          <Spinner />
          {t("파일을 비교하는 중")}
        </div>
      )}
      {data && (
        <>
          <p className="compare-summary">
            {(["added", "removed", "modified", "unchanged"] as const).map(
              (s) => (
                <span key={s}>
                  {changeLabel(s)} <strong>{number(data.summary[s])}</strong>
                </span>
              ),
            )}
          </p>
          <div className="compare-files-layout">
            <nav className="compare-files" aria-label={t("비교할 파일")}>
              {data.files.map((f) => (
                <button
                  key={f.path}
                  className={path === f.path ? "is-selected" : ""}
                  aria-current={path === f.path ? "true" : undefined}
                  onClick={() => setPath(f.path)}
                >
                  <span>{f.path}</span>
                  <small className={`change-${f.status}`}>
                    {changeLabel(f.status)}
                  </small>
                </button>
              ))}
            </nav>
            <div className="compare-file-detail">
              {file && (
                <>
                  <header>
                    <h5>{file.path}</h5>
                    <p className="workspace-feature-muted">
                      {bytes(file.beforeBytes)} → {bytes(file.afterBytes)}
                    </p>
                  </header>
                  <details className="compare-hashes">
                    <summary>{t("SHA-256 확인")}</summary>
                    <dl>
                      <dt>{t("이전")}</dt>
                      <dd>
                        <code>{file.beforeHash || "—"}</code>
                      </dd>
                      <dt>{t("이후")}</dt>
                      <dd>
                        <code>{file.afterHash || "—"}</code>
                      </dd>
                    </dl>
                  </details>
                  {file.status === "unchanged" ? (
                    <p className="compare-text-note">
                      {t("파일 내용이 동일합니다.")}
                    </p>
                  ) : file.textOmitted ? (
                    <p className="compare-text-note">
                      {t(
                        "텍스트 비교 범위를 초과했거나 텍스트 파일이 아닙니다. 위의 파일 크기와 SHA-256으로 변경을 확인하세요.",
                      )}
                      <br />
                      {t("텍스트 비교 한도: {bytes}, 버전당 {lines}줄", {
                        bytes: bytes(data.textLimitBytes),
                        lines: data.textLimitLines,
                      })}
                    </p>
                  ) : (
                    file.lines && (
                      <pre
                        className="compare-code"
                        tabIndex={0}
                        aria-label={t("파일의 추가 및 삭제 내용")}
                      >
                        <code>
                          {file.lines.map((line, index) => (
                            <span key={index} className={`diff-${line.kind}`}>
                              <span aria-hidden="true">
                                {line.kind === "added"
                                  ? "+"
                                  : line.kind === "removed"
                                    ? "−"
                                    : " "}{" "}
                              </span>
                              <span className="sr-only">
                                {line.kind === "added"
                                  ? t("추가된 줄")
                                  : line.kind === "removed"
                                    ? t("삭제된 줄")
                                    : t("변경 없는 줄")}
                                :{" "}
                              </span>
                              {line.text || " "}
                              {"\n"}
                            </span>
                          ))}
                        </code>
                      </pre>
                    )
                  )}
                </>
              )}
            </div>
          </div>
        </>
      )}
    </section>
  );
}
