import { useEffect, useId, useRef, useState } from "react";
import { Camera, Check, Trash2, Upload } from "lucide-react";
import { api, APIError, json } from "./api";
import { t, useLocale } from "./i18n";
import type { Organization, User } from "./types";
import { ErrorBox, Field, Spinner } from "./ui";
import "./ProfilePhotos.css";

function photoError(error: unknown) {
  if (error instanceof APIError) {
    if (["invalid_photo", "photo_too_large"].includes(error.code))
      return new Error(
        t(
          "JPEG 또는 PNG 파일을 선택해 주세요. 최대 5 MiB, 2048 × 2048픽셀까지 지원합니다.",
        ),
      );
    if (error.code === "photo_storage_full")
      return new Error(
        t(
          "사진 저장 공간이 가득 찼습니다. 사용하지 않는 사진을 삭제해 주세요.",
        ),
      );
    if (error.code === "invalid_profile_name")
      return new Error(t("이름은 줄바꿈 없이 1~100자로 입력해 주세요."));
  }
  return error;
}

function PhotoEditor({
  name,
  avatarUrl,
  organization,
  onSaved,
}: {
  name: string;
  avatarUrl?: string;
  organization: boolean;
  onSaved: (value: User | Organization) => void;
}) {
  useLocale();
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState("");
  const [failedImage, setFailedImage] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [message, setMessage] = useState<"saved" | "removed" | "">("");
  const endpoint = organization ? "/api/organization/photo" : "/api/me/photo";
  useEffect(() => {
    if (!file) {
      setPreview("");
      return;
    }
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);
  function reset() {
    setFile(null);
    if (input.current) input.current.value = "";
  }
  async function save(remove: boolean) {
    if (!remove && !file) return;
    setBusy(true);
    setError(null);
    setMessage("");
    try {
      const result = await api<{ user?: User; organization?: Organization }>(
        endpoint,
        {
          method: remove ? "DELETE" : "PUT",
          ...(remove
            ? {}
            : { body: file!, headers: { "Content-Type": file!.type } }),
        },
      );
      onSaved((organization ? result.organization : result.user)!);
      reset();
      setMessage(remove ? "removed" : "saved");
    } catch (e) {
      setError(photoError(e));
    } finally {
      setBusy(false);
    }
  }
  const image = preview || avatarUrl || "";
  return (
    <div className="profile-photo-editor" aria-busy={busy}>
      <div
        className={`profile-photo-preview${organization ? " organization" : ""}`}
      >
        {image && failedImage !== image ? (
          <img
            src={image}
            alt={
              organization ? t("조직 사진 미리보기") : t("프로필 사진 미리보기")
            }
            onError={() => setFailedImage(image)}
          />
        ) : (
          <span aria-hidden="true">
            {name.trim().slice(0, 2).toUpperCase() || <Camera size={28} />}
          </span>
        )}
      </div>
      <div className="profile-photo-controls">
        <p id={`${id}-hint`} className="profile-photo-hint">
          {t("JPEG 또는 PNG · 최대 5 MiB · 2048 × 2048픽셀")}
        </p>
        <div className="profile-photo-actions">
          <input
            ref={input}
            id={id}
            className="profile-photo-input"
            type="file"
            accept="image/jpeg,image/png"
            disabled={busy}
            aria-describedby={`${id}-hint`}
            onChange={(event) => {
              const selected = event.target.files?.[0];
              setError(null);
              setMessage("");
              if (!selected) return;
              if (
                !["image/jpeg", "image/png"].includes(selected.type) ||
                selected.size > 5 * 1024 * 1024 ||
                selected.size === 0
              ) {
                reset();
                setError(
                  new Error(
                    t(
                      "JPEG 또는 PNG 파일을 선택해 주세요. 최대 5 MiB, 2048 × 2048픽셀까지 지원합니다.",
                    ),
                  ),
                );
                return;
              }
              setFile(selected);
            }}
          />
          <label className="button secondary" htmlFor={id} aria-disabled={busy}>
            <Upload size={16} />
            {avatarUrl ? t("사진 변경") : t("사진 선택")}
          </label>
          {file && (
            <>
              <button
                type="button"
                className="button primary"
                disabled={busy}
                onClick={() => void save(false)}
              >
                {busy ? <Spinner /> : <Check size={16} />}
                {t("사진 저장")}
              </button>
              <button
                type="button"
                className="button secondary"
                disabled={busy}
                onClick={() => {
                  reset();
                  setError(null);
                }}
              >
                {t("취소")}
              </button>
            </>
          )}
          {avatarUrl && !file && (
            <button
              type="button"
              className="button secondary"
              disabled={busy}
              onClick={() => void save(true)}
            >
              <Trash2 size={16} />
              {t("사진 삭제")}
            </button>
          )}
        </div>
        {file && <p className="profile-photo-filename">{file.name}</p>}
        <p className="profile-photo-status" role="status" aria-live="polite">
          {message === "saved"
            ? t("사진을 저장했습니다.")
            : message === "removed"
              ? t("사진을 삭제했습니다.")
              : ""}
        </p>
        <ErrorBox error={error} />
      </div>
    </div>
  );
}

export function UserProfileSettings({
  user,
  onSaved,
}: {
  user: User;
  onSaved: (user: User) => void;
}) {
  useLocale();
  const [name, setName] = useState(user.name);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [saved, setSaved] = useState(false);
  useEffect(() => {
    setName(user.name);
  }, [user.name]);
  return (
    <section className="panel profile-settings">
      <div className="profile-settings-heading">
        <h2>{t("내 프로필")}</h2>
        <p>{t("이름과 사진은 조직 구성원에게 표시됩니다.")}</p>
      </div>
      <PhotoEditor
        name={user.name}
        avatarUrl={user.avatarUrl}
        organization={false}
        onSaved={(value) => onSaved(value as User)}
      />
      <form
        className="profile-name-form"
        onSubmit={async (event) => {
          event.preventDefault();
          setBusy(true);
          setError(null);
          setSaved(false);
          try {
            const result = await api<{ user: User }>(
              "/api/me",
              json("PATCH", { name }),
            );
            onSaved(result.user);
            setSaved(true);
          } catch (e) {
            setError(photoError(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        <Field label={t("이름")}>
          <input
            value={name}
            maxLength={100}
            required
            autoComplete="name"
            disabled={busy}
            onChange={(event) => {
              setName(event.target.value);
              setSaved(false);
            }}
          />
        </Field>
        <button
          className="button primary"
          disabled={busy || name.trim() === user.name || !name.trim()}
        >
          {busy ? <Spinner /> : <Check size={16} />}
          {t("이름 저장")}
        </button>
        <span role="status" className="profile-photo-status">
          {saved ? t("이름을 저장했습니다.") : ""}
        </span>
      </form>
      <ErrorBox error={error} />
    </section>
  );
}

export function OrganizationPhotoSettings({
  organization,
  onSaved,
}: {
  organization: Organization;
  onSaved: (organization: Organization) => void;
}) {
  useLocale();
  return (
    <section className="panel profile-settings">
      <div className="profile-settings-heading">
        <h2>{t("조직 사진")}</h2>
        <p>{t("조직을 알아보기 쉬운 사진이나 로고를 사용하세요.")}</p>
      </div>
      <PhotoEditor
        name={organization.name}
        avatarUrl={organization.avatarUrl}
        organization
        onSaved={(value) => onSaved(value as Organization)}
      />
    </section>
  );
}
