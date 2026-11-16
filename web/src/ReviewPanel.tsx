import { useCallback, useEffect, useId, useRef, useState } from "react";
import { Check, MessageSquare, RotateCcw } from "lucide-react";
import { api, date, json } from "./api";
import { ErrorBox, Field, Spinner } from "./ui";
import { number, t, useLocale } from "./i18n";
import type { Artifact, User, Version } from "./types";
import "./ReviewPanel.css";

type ReviewStatus = "draft" | "in_review" | "approved" | "changes_requested";
export interface VersionReview {
  artifactId: string;
  versionId: string;
  status: ReviewStatus;
  updatedBy: string;
  updatedByName: string;
  updatedAt: string;
}
export interface VersionComment {
  id: string;
  artifactId: string;
  versionId: string;
  parentId?: string;
  authorId: string;
  authorName: string;
  body: string;
  path?: string;
  anchor?: string;
  resolved: boolean;
  deleted: boolean;
  createdAt: string;
  updatedAt: string;
}
export interface ReviewPanelProps {
  artifact: Artifact & { reviewEnabled?: boolean; publishedVersionId?: string };
  version: Version;
  user: User | null;
  onChange?: () => void;
}
const statusLabel = (status: ReviewStatus) =>
  ({
    draft: t("초안"),
    in_review: t("검토 중"),
    approved: t("승인됨"),
    changes_requested: t("수정 요청"),
  })[status];

export function ReviewPanel({
  artifact,
  version,
  user,
  onChange,
}: ReviewPanelProps) {
  useLocale();
  const headingId = useId();
  const key = `${artifact.id}:${version.id}`;
  const currentKey = useRef(key);
  currentKey.current = key;
  const [review, setReview] = useState<VersionReview | null>(null);
  const [comments, setComments] = useState<VersionComment[]>([]);
  const [status, setStatus] = useState<ReviewStatus>("draft");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState("");
  const [body, setBody] = useState("");
  const [path, setPath] = useState("");
  const [anchor, setAnchor] = useState("");
  const [showResolved, setShowResolved] = useState(true);
  const base = `/api/artifacts/${encodeURIComponent(artifact.id)}`;
  const manager =
    !!user && (user.id === artifact.ownerId || user.role === "admin");
  const load = useCallback(
    async (signal?: AbortSignal) => {
      const query = `?versionId=${encodeURIComponent(version.id)}`;
      const [r, c] = await Promise.all([
        api<{ review: VersionReview }>(`${base}/review${query}`, { signal }),
        api<{ comments: VersionComment[] }>(`${base}/comments${query}`, {
          signal,
        }),
      ]);
      if (!signal?.aborted && currentKey.current === key) {
        setReview(r.review);
        setStatus(r.review.status);
        setComments(c.comments);
      }
    },
    [base, version.id, key],
  );
  useEffect(() => {
    const controller = new AbortController();
    setReview(null);
    setComments([]);
    setBody("");
    setPath("");
    setAnchor("");
    setError(null);
    setNotice("");
    setBusy(false);
    setLoading(true);
    if (!user) {
      setLoading(false);
      return () => controller.abort();
    }
    void load(controller.signal)
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setError(e);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [load, user?.id]);
  async function change(
    action: () => Promise<unknown>,
    message: string,
    after?: () => void,
  ) {
    setBusy(true);
    setError(null);
    setNotice("");
    try {
      await action();
      if (currentKey.current !== key) return;
      after?.();
      await load();
      if (currentKey.current !== key) return;
      setNotice(message);
      onChange?.();
    } catch (e) {
      if (currentKey.current === key) setError(e);
    } finally {
      if (currentKey.current === key) setBusy(false);
    }
  }
  if (!user)
    return (
      <p className="review-muted">
        {t("댓글과 검토 기록은 로그인한 멤버만 볼 수 있습니다.")}
      </p>
    );
  const roots = comments.filter(
    (c) => !c.parentId && (showResolved || !c.resolved),
  );
  return (
    <section
      className="review-panel"
      aria-labelledby={headingId}
      aria-busy={loading || busy}
    >
      <header className="review-heading">
        <div>
          <h3 id={headingId}>{t("검토와 댓글")}</h3>
          <p>
            {t("버전 {number}에 연결된 기록입니다.", {
              number: version.number,
            })}
          </p>
        </div>
        {review && (
          <span className={`review-state review-state-${review.status}`}>
            {statusLabel(review.status)}
          </span>
        )}
      </header>
      {manager && <ReviewerSettings artifact={artifact} onChange={onChange} />}
      <ErrorBox error={error} />
      {notice && (
        <p className="review-notice" role="status">
          <Check size={16} />
          {t(notice)}
        </p>
      )}
      {loading ? (
        <div className="review-loading">
          <Spinner />
          {t("검토 기록을 불러오는 중")}
        </div>
      ) : (
        <>
          {review && (
            <div className="review-decision">
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  void change(
                    () =>
                      api(
                        `${base}/review`,
                        json("PUT", { versionId: version.id, status }),
                      ),
                    t("검토 상태를 저장했습니다."),
                  );
                }}
              >
                <Field label={t("검토 상태")}>
                  <select
                    value={status}
                    disabled={busy}
                    onChange={(e) => setStatus(e.target.value as ReviewStatus)}
                  >
                    {(
                      [
                        "draft",
                        "in_review",
                        "approved",
                        "changes_requested",
                      ] as const
                    ).map((s) => (
                      <option key={s} value={s}>
                        {statusLabel(s)}
                      </option>
                    ))}
                  </select>
                </Field>
                <button
                  className="button secondary"
                  disabled={busy || status === review.status}
                >
                  {t("상태 저장")}
                </button>
              </form>
              {review.updatedByName && (
                <p className="review-muted">
                  {review.updatedByName} · {date(review.updatedAt)}
                </p>
              )}
              {manager && artifact.reviewEnabled && (
                <div className="review-publish">
                  <p>
                    {artifact.publishedVersionId === version.id
                      ? t("현재 게시된 버전입니다.")
                      : t("승인 후 이 버전을 게시하면 공유 링크에 반영됩니다.")}
                  </p>
                  {artifact.publishedVersionId !== version.id && (
                    <button
                      className="button primary"
                      disabled={busy || review.status !== "approved"}
                      onClick={() =>
                        void change(
                          () =>
                            api(
                              `${base}/promote`,
                              json("POST", {
                                versionId: version.id,
                                basePublishedVersionId:
                                  artifact.publishedVersionId || "",
                              }),
                            ),
                          t("승인된 버전을 게시했습니다."),
                        )
                      }
                    >
                      {t("이 버전 게시")}
                    </button>
                  )}
                </div>
              )}
            </div>
          )}
          <div className="review-thread-heading">
            <h4>
              <MessageSquare size={17} />
              {t("댓글")}{" "}
              <span>{number(comments.filter((c) => !c.deleted).length)}</span>
            </h4>
            <label className="review-check">
              <input
                type="checkbox"
                checked={showResolved}
                onChange={(e) => setShowResolved(e.target.checked)}
              />
              {t("해결된 대화 표시")}
            </label>
          </div>
          {roots.length === 0 && (
            <p className="review-empty">
              {comments.some((c) => !c.parentId)
                ? t("표시할 미해결 대화가 없습니다.")
                : t("이 버전의 첫 번째 의견을 남겨 주세요.")}
            </p>
          )}
          <div className="review-threads">
            {roots.map((root) => (
              <ReviewThread
                key={`${key}:${root.id}`}
                root={root}
                replies={comments.filter((c) => c.parentId === root.id)}
                user={user}
                manager={manager}
                busy={busy}
                onReply={(reply, clear) =>
                  void change(
                    () =>
                      api(
                        `${base}/comments`,
                        json("POST", {
                          versionId: version.id,
                          parentId: root.id,
                          body: reply,
                        }),
                      ),
                    t("답글을 남겼습니다."),
                    clear,
                  )
                }
                onEdit={(id, text, clear) =>
                  void change(
                    () =>
                      api(
                        `${base}/comments/${id}`,
                        json("PATCH", { body: text }),
                      ),
                    t("댓글을 수정했습니다."),
                    clear,
                  )
                }
                onDelete={(id) =>
                  void change(
                    () => api(`${base}/comments/${id}`, json("DELETE")),
                    t("댓글을 삭제했습니다."),
                  )
                }
                onResolve={() =>
                  void change(
                    () =>
                      api(
                        `${base}/comments/${root.id}`,
                        json("PATCH", { resolved: !root.resolved }),
                      ),
                    root.resolved
                      ? t("대화를 다시 열었습니다.")
                      : t("대화를 해결했습니다."),
                  )
                }
              />
            ))}
          </div>
          <form
            className="review-compose"
            onSubmit={(e) => {
              e.preventDefault();
              void change(
                () =>
                  api(
                    `${base}/comments`,
                    json("POST", { versionId: version.id, body, path, anchor }),
                  ),
                t("댓글을 남겼습니다."),
                () => {
                  setBody("");
                  setPath("");
                  setAnchor("");
                },
              );
            }}
          >
            <Field label={t("새 댓글")}>
              <textarea
                required
                maxLength={20000}
                rows={3}
                value={body}
                disabled={busy}
                onChange={(e) => setBody(e.target.value)}
                placeholder={t("이 버전에서 확인할 점을 남겨 주세요.")}
              />
            </Field>
            <details className="review-location">
              <summary>{t("파일 위치 지정 (선택)")}</summary>
              <div>
                <Field label={t("파일")}>
                  <select
                    value={path}
                    disabled={busy}
                    onChange={(e) => {
                      setPath(e.target.value);
                      if (!e.target.value) setAnchor("");
                    }}
                  >
                    <option value="">{t("전체 버전")}</option>
                    {version.files.map((f) => (
                      <option key={f.path} value={f.path}>
                        {f.path}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label={t("위치 또는 선택한 문구")}>
                  <input
                    value={anchor}
                    maxLength={1000}
                    disabled={busy || !path}
                    onChange={(e) => setAnchor(e.target.value)}
                  />
                </Field>
              </div>
            </details>
            <div className="review-compose-footer">
              <span className="review-muted">
                {t("댓글은 선택한 버전에만 저장됩니다.")}
              </span>
              <button
                className="button primary"
                disabled={busy || !body.trim()}
              >
                {busy ? <Spinner /> : t("댓글 남기기")}
              </button>
            </div>
          </form>
        </>
      )}
    </section>
  );
}

function ReviewThread({
  root,
  replies,
  user,
  manager,
  busy,
  onReply,
  onEdit,
  onDelete,
  onResolve,
}: {
  root: VersionComment;
  replies: VersionComment[];
  user: User;
  manager: boolean;
  busy: boolean;
  onReply: (body: string, clear: () => void) => void;
  onEdit: (id: string, body: string, clear: () => void) => void;
  onDelete: (id: string) => void;
  onResolve: () => void;
}) {
  const [replying, setReplying] = useState(false);
  const [reply, setReply] = useState("");
  return (
    <article className={`review-thread${root.resolved ? " is-resolved" : ""}`}>
      <div className="review-thread-state">
        {root.resolved && (
          <span>
            <Check size={14} />
            {t("해결됨")}
          </span>
        )}
        {(root.authorId === user.id || manager) && (
          <button
            type="button"
            className="button ghost compact"
            disabled={busy}
            onClick={onResolve}
          >
            {root.resolved ? <RotateCcw size={14} /> : <Check size={14} />}
            {root.resolved ? t("다시 열기") : t("해결")}
          </button>
        )}
      </div>
      <ReviewComment
        comment={root}
        user={user}
        busy={busy}
        onEdit={onEdit}
        onDelete={onDelete}
      />
      <div className="review-replies">
        {replies.map((c) => (
          <ReviewComment
            key={c.id}
            comment={c}
            user={user}
            busy={busy}
            onEdit={onEdit}
            onDelete={onDelete}
          />
        ))}
      </div>
      {!root.resolved &&
        (replying ? (
          <form
            className="review-reply-form"
            onSubmit={(e) => {
              e.preventDefault();
              onReply(reply, () => {
                setReply("");
                setReplying(false);
              });
            }}
          >
            <Field label={t("답글")}>
              <textarea
                required
                maxLength={20000}
                rows={2}
                value={reply}
                disabled={busy}
                onChange={(e) => setReply(e.target.value)}
                autoFocus
              />
            </Field>
            <div>
              <button
                type="button"
                className="button ghost compact"
                disabled={busy}
                onClick={() => setReplying(false)}
              >
                {t("취소")}
              </button>
              <button
                className="button secondary compact"
                disabled={busy || !reply.trim()}
              >
                {t("답글 남기기")}
              </button>
            </div>
          </form>
        ) : (
          <button
            type="button"
            className="button ghost compact"
            disabled={busy}
            onClick={() => setReplying(true)}
          >
            {t("답글")}
          </button>
        ))}
    </article>
  );
}

function ReviewComment({
  comment: c,
  user,
  busy,
  onEdit,
  onDelete,
}: {
  comment: VersionComment;
  user: User;
  busy: boolean;
  onEdit: (id: string, body: string, clear: () => void) => void;
  onDelete: (id: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(c.body);
  const [deleting, setDeleting] = useState(false);
  const canEdit =
    !c.deleted && (user.id === c.authorId || user.role === "admin");
  return (
    <div className="review-comment">
      <div className="review-comment-meta">
        <strong>{c.authorName}</strong>
        <time dateTime={c.createdAt}>{date(c.createdAt)}</time>
      </div>
      {c.path && (
        <p className="review-anchor">
          <code>{c.path}</code>
          {c.anchor && <span>{c.anchor}</span>}
        </p>
      )}
      {editing ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            onEdit(c.id, text, () => setEditing(false));
          }}
        >
          <Field label={t("댓글 수정")}>
            <textarea
              required
              rows={3}
              maxLength={20000}
              value={text}
              disabled={busy}
              onChange={(e) => setText(e.target.value)}
              autoFocus
            />
          </Field>
          <div className="review-inline-actions">
            <button
              type="button"
              className="button ghost compact"
              disabled={busy}
              onClick={() => setEditing(false)}
            >
              {t("취소")}
            </button>
            <button
              className="button secondary compact"
              disabled={busy || !text.trim()}
            >
              {t("저장")}
            </button>
          </div>
        </form>
      ) : (
        <p className={c.deleted ? "review-muted" : "review-comment-body"}>
          {c.deleted ? t("삭제된 댓글입니다.") : c.body}
        </p>
      )}
      {canEdit && !editing && (
        <div className="review-inline-actions">
          {deleting ? (
            <>
              <span>{t("이 댓글을 삭제할까요?")}</span>
              <button
                type="button"
                className="button ghost compact"
                disabled={busy}
                onClick={() => setDeleting(false)}
              >
                {t("취소")}
              </button>
              <button
                type="button"
                className="button danger compact"
                disabled={busy}
                onClick={() => onDelete(c.id)}
              >
                {t("삭제")}
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                className="button ghost compact"
                disabled={busy}
                onClick={() => {
                  setText(c.body);
                  setEditing(true);
                }}
              >
                {t("편집")}
              </button>
              <button
                type="button"
                className="button ghost compact"
                disabled={busy}
                onClick={() => setDeleting(true)}
              >
                {t("삭제")}
              </button>
            </>
          )}
        </div>
      )}
    </div>
  );
}

export function ReviewerSettings({
  artifact,
  onChange,
}: {
  artifact: Artifact & { reviewerIds?: string[] };
  onChange?: () => void;
}) {
  useLocale();
  const [members, setMembers] = useState<User[]>([]);
  const [ids, setIds] = useState<string[]>(artifact.reviewerIds || []);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const reviewerKey = (artifact.reviewerIds || []).join(":");
  useEffect(() => {
    const controller = new AbortController();
    setIds(artifact.reviewerIds || []);
    setSaved(false);
    setError(null);
    void api<{ members: User[] }>("/api/members", { signal: controller.signal })
      .then((out) => {
        if (!controller.signal.aborted)
          setMembers(out.members.filter((m) => !m.disabled));
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setError(e);
      });
    return () => controller.abort();
  }, [artifact.id, reviewerKey]);
  return (
    <details className="review-reviewers">
      <summary>{t("검토자 지정")}</summary>
      <p className="review-muted">
        {t(
          "지정한 멤버는 초안을 검토할 수 있습니다. 아티팩트 공유 권한도 함께 부여해 주세요.",
        )}
      </p>
      <ErrorBox error={error} />
      <form
        onSubmit={(e) => {
          e.preventDefault();
          setBusy(true);
          setError(null);
          setSaved(false);
          void api(
            `/api/artifacts/${encodeURIComponent(artifact.id)}/reviewers`,
            json("PATCH", { reviewerIds: ids }),
          )
            .then(() => {
              setSaved(true);
              onChange?.();
            })
            .catch(setError)
            .finally(() => setBusy(false));
        }}
      >
        <fieldset disabled={busy}>
          <legend className="sr-only">{t("검토자")}</legend>
          <div className="review-member-list">
            {members.map((member) => (
              <label key={member.id}>
                <input
                  type="checkbox"
                  checked={ids.includes(member.id)}
                  onChange={(e) => {
                    setSaved(false);
                    setIds((old) =>
                      e.target.checked
                        ? [...old, member.id]
                        : old.filter((id) => id !== member.id),
                    );
                  }}
                />
                <span>
                  <strong>{member.name}</strong>
                  <small>{member.email}</small>
                </span>
              </label>
            ))}
          </div>
        </fieldset>
        <div className="review-compose-footer">
          {saved ? (
            <span role="status" className="review-notice">
              <Check size={14} />
              {t("검토자를 저장했습니다.")}
            </span>
          ) : (
            <span className="review-muted">
              {t("검토자 지정은 공유 범위를 넓히지 않습니다.")}
            </span>
          )}
          <button className="button secondary" disabled={busy}>
            {busy ? <Spinner /> : t("검토자 저장")}
          </button>
        </div>
      </form>
    </details>
  );
}
