import { Select } from "./Select";
import { t as translate, useLocale } from "./i18n";
import { AppearanceControls } from "./AppearanceControls";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Archive,
  FolderOpen,
  Star,
  Trash2,
  DatabaseBackup,
  ArrowUpRight,
  Globe,
  Grid2X2,
  HardDrive,
  History,
  Info,
  Layers,
  List,
  LogOut,
  Menu,
  Plus,
  Search,
  Share2,
  Settings,
  ShieldCheck,
  Sparkles,
  UserRound,
  Users,
  X,
} from "lucide-react";
import { api, bytes, json, setCSRF } from "./api";
import type { Artifact, Policy, Session } from "./types";
import { CollectionsView } from "./WorkspaceFeatures";
import { TrashView, BackupSettings, GuestViewer } from "./OperationsFeatures";
import Auth, { Brand, PasswordChange } from "./Auth";
import {
  ArtifactCard,
  DetailPanel,
  LibraryEmpty,
  SharePanel,
  UploadDialog,
  visibilityLabels,
} from "./Artifacts";
import {
  AccountPage,
  AgentsPage,
  AuditPage,
  DomainsPage,
  MembersPage,
  SettingsPage,
  StoragePage,
  useResource,
} from "./Admin";
import { Avatar, Empty, ErrorBox, Spinner } from "./ui";

type View =
  | "library"
  | "mine"
  | "shared"
  | "archive"
  | "collections"
  | "favorites"
  | "trash"
  | "backups"
  | "members"
  | "storage"
  | "domains"
  | "agents"
  | "settings"
  | "audit"
  | "account";
const getNav = () => [
  { id: "library", label: translate("전체 아티팩트"), icon: Layers },
  { id: "mine", label: translate("내 아티팩트"), icon: UserRound },
  { id: "shared", label: translate("조직 공유함"), icon: Users },
  { id: "favorites", label: translate("즐겨찾기"), icon: Star },
  { id: "collections", label: translate("컬렉션"), icon: FolderOpen },
  { id: "trash", label: translate("휴지통"), icon: Trash2 },
  { id: "archive", label: translate("아카이브"), icon: Archive },
];
const getManagement = () => [
  { id: "members", label: translate("구성원"), icon: Users },
  { id: "storage", label: translate("저장소와 보관"), icon: HardDrive },
  { id: "domains", label: translate("도메인"), icon: Globe },
  { id: "agents", label: translate("에이전트"), icon: Sparkles },
  { id: "settings", label: translate("조직 설정"), icon: Settings },
  { id: "backups", label: translate("백업 및 복원"), icon: DatabaseBackup },
  { id: "audit", label: translate("변경 기록"), icon: History },
  { id: "account", label: translate("내 계정"), icon: UserRound },
];

export default function App() {
  useLocale();
  const [session, setSession] = useState<Session | null>(null);
  const [setup, setSetup] = useState(false);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [login, setLogin] = useState(false);
  const viewer = location.pathname.startsWith("/s/");
  useEffect(() => {
    function expired() {
      setSession(null);
      setCSRF("");
    }
    window.addEventListener("folio:session-expired", expired);
    return () => window.removeEventListener("folio:session-expired", expired);
  }, []);
  useEffect(() => {
    if (location.pathname.startsWith("/g/")) {
      setReady(true);
      return;
    }
    let active = true;
    api<{ needsSetup: boolean }>("/api/bootstrap")
      .then(async (info) => {
        if (!active) return;
        setSetup(info.needsSetup);
        if (!info.needsSetup) {
          try {
            const s = await api<Session>("/api/me");
            if (active) {
              setSession(s);
              setCSRF(s.csrfToken);
            }
          } catch {
            /* A visitor can remain anonymous. */
          }
        }
      })
      .catch((e) => {
        if (active) setError(e);
      })
      .finally(() => {
        if (active) setReady(true);
      });
    return () => {
      active = false;
    };
  }, []);
  function authenticated(s: Session) {
    setSession(s);
    setSetup(false);
    setLogin(false);
    setCSRF(s.csrfToken);
  }
  if (!ready)
    return (
      <div className="boot-screen">
        <Brand />
        <Spinner />
        <p>{translate("작업 공간을 준비하고 있습니다.")}</p>
      </div>
    );
  if (error)
    return (
      <div className="boot-screen">
        <Brand />
        <ErrorBox error={error} />
        <button className="button secondary" onClick={() => location.reload()}>
          {translate("다시 시도")}
        </button>
      </div>
    );
  if (location.pathname.startsWith("/g/"))
    return <GuestViewer id={decodeURIComponent(location.pathname.slice(3))} />;
  if (setup || login || (!session && !viewer))
    return <Auth setup={setup} onLogin={authenticated} />;
  if (session?.user.mustChangePassword)
    return (
      <PasswordChange onChanged={(s) => setSession({ ...session, ...s })} />
    );
  if (viewer)
    return <Viewer session={session} onLogin={() => setLogin(true)} />;
  return (
    <Workspace
      session={session!}
      onSession={setSession}
      onLogout={async () => {
        await api("/api/auth/logout", json("POST"));
        setCSRF("");
        setSession(null);
      }}
    />
  );
}

function Viewer({
  session,
  onLogin,
}: {
  session: Session | null;
  onLogin: () => void;
}) {
  useLocale();
  const slug = decodeURIComponent(location.pathname.slice(3));
  const r = useResource<{ artifact: Artifact }>(
    `/api/share/${encodeURIComponent(slug)}${location.search}`,
    session ? 1 : 0,
  );
  const [url, setURL] = useState("");
  const [launchError, setLaunchError] = useState<unknown>(null);
  const [dialog, setDialog] = useState<"detail" | "share" | "upload" | null>(
    null,
  );
  const [managedArtifact, setManagedArtifact] = useState<Artifact | null>(null);
  const [managementError, setManagementError] = useState<unknown>(null);
  const artifact = r.data?.artifact;
  const canManage =
    !!session &&
    !!artifact &&
    (session.user.role === "admin" || session.user.id === artifact.ownerId);
  const closeDialog = useCallback(() => setDialog(null), []);
  async function refreshManagement() {
    if (!session || !artifact) return;
    const result = await api<{ artifact: Artifact }>(
      `/api/artifacts/${artifact.id}`,
    );
    setManagedArtifact(result.artifact);
  }
  async function openManagement(next: "detail" | "share" | "upload") {
    setManagementError(null);
    try {
      await refreshManagement();
      setDialog(next);
    } catch (error) {
      setManagementError(error);
    }
  }
  function managementChanged() {
    void refreshManagement().catch(setManagementError);
    r.reload();
  }
  useEffect(() => {
    if (!r.data) return;
    let active = true;
    const query = new URLSearchParams(location.search);
    const hostname = query.get("domain");
    const versionId = query.get("version") || r.data.artifact.currentVersionId;
    api<{ url: string }>(
      `/api/artifacts/${r.data.artifact.id}/launch`,
      json("POST", {
        ...(hostname ? { hostname } : {}),
        ...(versionId ? { versionId } : {}),
      }),
    )
      .then((result) => {
        if (active) setURL(result.url);
      })
      .catch((e) => {
        if (active) setLaunchError(e);
      });
    return () => {
      active = false;
    };
  }, [r.data]);
  return (
    <main className="viewer">
      <header className="viewer-header">
        <a href="/" aria-label={translate("Folio 작업 공간")}>
          <Brand />
        </a>
        <span>{r.data?.artifact.title || translate("공유된 아티팩트")}</span>
        <div>
          {session && artifact && (
            <>
              <button
                className="icon-button"
                aria-label={translate("아티팩트 상세")}
                title={translate("아티팩트 상세")}
                onClick={() => void openManagement("detail")}
              >
                <Info size={18} />
              </button>
              <button
                className="button primary compact"
                onClick={() => void openManagement("share")}
              >
                <Share2 size={15} />
                {translate("공유")}
              </button>
            </>
          )}
          {r.data && (
            <small className="scope-label">
              {visibilityLabels[r.data.artifact.visibility]}
            </small>
          )}
          {session ? (
            <a className="button secondary compact" href="/">
              {translate("작업 공간")}
              <ArrowUpRight size={15} />
            </a>
          ) : (
            <button className="button secondary compact" onClick={onLogin}>
              {translate("로그인")}
            </button>
          )}
        </div>
      </header>
      <ErrorBox error={managementError} />
      {r.loading && !r.data ? (
        <div className="boot-screen">
          <Spinner />
        </div>
      ) : r.error ? (
        <Empty
          icon={<ShieldCheck size={32} />}
          heading={translate("이 아티팩트를 열 수 없습니다")}
          description={translate(
            "접근 권한이 없거나 공유가 종료되었습니다. 계정으로 로그인해 권한을 확인하세요.",
          )}
          action={
            !session ? (
              <button className="button primary" onClick={onLogin}>
                {translate("로그인")}
              </button>
            ) : (
              <a className="button secondary" href="/">
                {translate("작업 공간으로")}
              </a>
            )
          }
        />
      ) : launchError ? (
        <ErrorBox error={launchError} />
      ) : url ? (
        <iframe
          className="viewer-frame"
          title={r.data?.artifact.title || translate("아티팩트")}
          src={url}
          sandbox="allow-scripts allow-same-origin allow-forms allow-downloads"
        />
      ) : (
        <div className="boot-screen">
          <Spinner />
        </div>
      )}
      {session && managedArtifact && dialog === "detail" && (
        <DetailPanel
          artifact={managedArtifact}
          user={session.user}
          canManage={canManage}
          onClose={closeDialog}
          onShare={() => setDialog("share")}
          onUpload={() => setDialog("upload")}
          onChanged={managementChanged}
        />
      )}
      {session && managedArtifact && dialog === "share" && (
        <SharePanel
          artifact={managedArtifact}
          canManage={canManage}
          onClose={closeDialog}
          onSaved={managementChanged}
        />
      )}
      {canManage && managedArtifact && dialog === "upload" && (
        <UploadDialog
          existing={managedArtifact}
          onClose={closeDialog}
          onSaved={managementChanged}
        />
      )}
    </main>
  );
}

function Workspace({
  session,
  onSession,
  onLogout,
}: {
  session: Session;
  onSession: (s: Session) => void;
  onLogout: () => Promise<void>;
}) {
  useLocale();
  const [view, setView] = useState<View>(() => {
    const hash = location.hash.slice(1);
    return [...getNav(), ...getManagement()].some((n) => n.id === hash)
      ? (hash as View)
      : "library";
  });
  const [mobile, setMobile] = useState(false);
  const [compactNav, setCompactNav] = useState(
    () => window.matchMedia("(max-width: 900px)").matches,
  );
  const sidebarRef = useRef<HTMLElement>(null);
  const closeNavRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const media = window.matchMedia("(max-width: 900px)");
    const changed = () => {
      setCompactNav(media.matches);
      if (!media.matches) setMobile(false);
    };
    media.addEventListener("change", changed);
    return () => media.removeEventListener("change", changed);
  }, []);
  useEffect(() => {
    if (!mobile || !compactNav) return;
    const previous = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeNavRef.current?.focus();
    function trap(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        setMobile(false);
      }
      if (e.key !== "Tab") return;
      const items = [
        ...(sidebarRef.current?.querySelectorAll<HTMLElement>(
          'a[href],button:not(:disabled),input:not(:disabled),[tabindex="0"]',
        ) || []),
      ].filter((el) => el.offsetParent !== null);
      const first = items[0],
        last = items.at(-1);
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last?.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first?.focus();
      }
    }
    document.addEventListener("keydown", trap);
    return () => {
      document.removeEventListener("keydown", trap);
      document.body.style.overflow = overflow;
      previous?.focus();
    };
  }, [mobile, compactNav]);
  const [query, setQuery] = useState("");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState("updated");
  const [layout, setLayout] = useState("grid");
  const [revision, setRevision] = useState(0);
  const [share, setShare] = useState<Artifact | null>(null);
  const [upload, setUpload] = useState<Artifact | true | null>(null);
  const [toast, setToast] = useState("");
  const [error, setError] = useState<unknown>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const timer = setTimeout(() => setSearch(query), 220);
    return () => clearTimeout(timer);
  }, [query]);
  const mode = ["mine", "shared", "archive"].includes(view) ? view : "";
  const r = useResource<{
    artifacts: Artifact[];
    usage: { usedBytes: number; quotaBytes: number };
    policy: Policy;
  }>(
    `/api/artifacts?${new URLSearchParams({ ...(mode ? { view: mode } : {}), q: search, sort, ...(view === "favorites" ? { favorite: "true" } : {}) })}`,
    revision,
  );
  const isAdmin = session.user.role === "admin";
  const canManage = (a: Artifact) => isAdmin || a.ownerId === session.user.id;
  const closeShare = useCallback(() => setShare(null), []);
  const closeUpload = useCallback(() => setUpload(null), []);
  function go(v: View) {
    if (
      ["library", "mine", "shared", "favorites", "archive", "trash"].includes(v)
    )
      setRevision((n) => n + 1);
    setView(v);
    location.hash = v;
    setMobile(false);
    setQuery("");
    setShare(null);
  }
  function changed(message = translate("변경 사항을 저장했습니다.")) {
    setRevision((n) => n + 1);
    setToast(message);
    setTimeout(() => setToast(""), 3500);
  }
  useEffect(() => {
    function key(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        if (
          !["library", "mine", "shared", "archive", "favorites"].includes(view)
        )
          go("library");
        setTimeout(() => searchRef.current?.focus(), 0);
      }
      if (e.key === "Escape") setMobile(false);
    }
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [view]);
  useEffect(() => {
    function back() {
      const next = location.hash.slice(1);
      if ([...getNav(), ...getManagement()].some((n) => n.id === next)) {
        setView(next as View);
        if (
          [
            "library",
            "mine",
            "shared",
            "favorites",
            "archive",
            "trash",
          ].includes(next)
        )
          setRevision((n) => n + 1);
      }
    }
    window.addEventListener("hashchange", back);
    return () => window.removeEventListener("hashchange", back);
  }, []);
  const artifacts = r.data?.artifacts || [];
  const library = [
    "library",
    "mine",
    "shared",
    "archive",
    "favorites",
  ].includes(view);
  const title =
    view === "favorites"
      ? translate("즐겨찾기")
      : view === "mine"
        ? translate("내 아티팩트")
        : view === "shared"
          ? translate("조직 공유함")
          : view === "archive"
            ? translate("아카이브")
            : translate("전체 아티팩트");
  return (
    <div className="app-shell">
      {mobile && (
        <button
          className="nav-scrim"
          aria-label={translate("탐색 닫기")}
          tabIndex={-1}
          aria-hidden="true"
          onClick={() => setMobile(false)}
        />
      )}
      <aside
        ref={sidebarRef}
        id="workspace-navigation"
        className={`sidebar ${mobile ? "open" : ""}`}
        inert={compactNav && !mobile}
        role={compactNav && mobile ? "dialog" : undefined}
        aria-modal={compactNav && mobile ? true : undefined}
        aria-label={translate("작업 공간 탐색")}
      >
        <div className="sidebar-heading">
          <a
            className="brand-link"
            href="/#library"
            onClick={(e) => {
              e.preventDefault();
              go("library");
            }}
          >
            <Brand />
          </a>
          <button
            ref={closeNavRef}
            className="icon-button sidebar-close"
            aria-label={translate("메뉴 닫기")}
            onClick={() => setMobile(false)}
          >
            <X size={20} />
          </button>
        </div>
        <button
          className="workspace-switch"
          onClick={() => go(isAdmin ? "settings" : "account")}
          aria-label={translate("작업 공간 설정")}
        >
          <Avatar
            className="workspace-monogram"
            name={session.organization.name}
            src={session.organization.avatarUrl}
          />
          <b>{session.organization.name}</b>
          <Settings size={15} />
        </button>
        <nav
          aria-label={translate("작업 공간 탐색")}
          className="primary-navigation"
        >
          {[
            { id: "library", label: translate("라이브러리"), icon: Layers },
            { id: "collections", label: translate("컬렉션"), icon: FolderOpen },
            { id: "agents", label: translate("에이전트"), icon: Sparkles },
            ...(isAdmin
              ? [{ id: "settings", label: translate("설정"), icon: Settings }]
              : []),
          ].map((item) => {
            const active =
              item.id === "library"
                ? library || view === "trash"
                : item.id === "settings"
                  ? isAdmin &&
                    !["agents", "account", "collections"].includes(view) &&
                    !library &&
                    view !== "trash"
                  : item.id === view;
            return (
              <button
                key={item.id}
                className={active ? "active" : ""}
                aria-current={active ? "page" : undefined}
                onClick={() => go(item.id as View)}
              >
                <item.icon size={17} />
                {item.label}
              </button>
            );
          })}
        </nav>
        <div className="sidebar-bottom">
          <button
            className="capacity-inline"
            onClick={() => go(isAdmin ? "storage" : "account")}
            title={translate("보관 용량")}
            aria-label={translate("보관 용량")}
          >
            <span>
              <HardDrive size={15} />
              {translate("보관 용량")}
            </span>
            <small>
              {r.data
                ? `${bytes(r.data.usage.usedBytes)} / ${bytes(r.data.usage.quotaBytes)}`
                : "—"}
            </small>
            <span
              className="capacity-track"
              role="meter"
              aria-label={translate("보관 용량")}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={Math.min(
                100,
                Math.round(
                  ((r.data?.usage.usedBytes || 0) /
                    (r.data?.usage.quotaBytes || 1)) *
                    100,
                ),
              )}
              aria-valuetext={
                r.data
                  ? `${bytes(r.data.usage.usedBytes)} / ${bytes(r.data.usage.quotaBytes)}`
                  : undefined
              }
            >
              <i
                style={{
                  width: `${Math.min(100, ((r.data?.usage.usedBytes || 0) / (r.data?.usage.quotaBytes || 1)) * 100)}%`,
                }}
              />
            </span>
          </button>
          <div className="sidebar-user">
            <button className="sidebar-profile" onClick={() => go("account")}>
              <Avatar name={session.user.name} src={session.user.avatarUrl} />
              <span>
                <b>{session.user.name}</b>
                <small>
                  {isAdmin ? translate("관리자") : translate("구성원")}
                </small>
              </span>
            </button>
            <button
              className="icon-button"
              title={translate("로그아웃")}
              aria-label={translate("로그아웃")}
              onClick={() => void onLogout().catch(setError)}
            >
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </aside>
      <div className="main-shell" inert={compactNav && mobile}>
        <header className="workspace-topbar">
          <button
            className="icon-button workspace-menu"
            aria-label={translate("메뉴 열기")}
            aria-expanded={mobile}
            aria-controls="workspace-navigation"
            onClick={() => setMobile(true)}
          >
            <Menu size={20} />
          </button>
          <span className="workspace-breadcrumb">
            <span>{session.organization.name}</span>
            <span className="breadcrumb-divider">/</span>
            <b>
              {
                [...getNav(), ...getManagement()].find((n) => n.id === view)
                  ?.label
              }
            </b>
          </span>
          <div className="workspace-header-actions">
            <AppearanceControls />
            <button
              className="topbar-account"
              onClick={() => go("account")}
              aria-label={translate("내 계정 열기")}
              title={translate("내 계정")}
            >
              <Avatar name={session.user.name} src={session.user.avatarUrl} />
            </button>
          </div>
        </header>
        <main className={`workspace-main ${library ? "library-main" : ""}`}>
          <ErrorBox error={error} />
          {isAdmin &&
            [
              "settings",
              "members",
              "storage",
              "domains",
              "backups",
              "audit",
            ].includes(view) && (
              <nav
                className="settings-navigation"
                aria-label={translate("설정 탐색")}
              >
                {getManagement()
                  .filter((n) => !["agents", "account"].includes(n.id))
                  .sort(
                    (a, b) =>
                      [
                        "settings",
                        "members",
                        "storage",
                        "domains",
                        "backups",
                        "audit",
                      ].indexOf(a.id) -
                      [
                        "settings",
                        "members",
                        "storage",
                        "domains",
                        "backups",
                        "audit",
                      ].indexOf(b.id),
                  )
                  .map((n) => (
                    <button
                      key={n.id}
                      onClick={() => go(n.id as View)}
                      aria-current={view === n.id ? "page" : undefined}
                      className={view === n.id ? "active" : ""}
                    >
                      {n.label}
                    </button>
                  ))}
              </nav>
            )}
          {view === "trash" && (
            <nav
              className="library-filters"
              aria-label={translate("라이브러리 필터")}
            >
              {getNav()
                .filter((n) => n.id !== "collections")
                .map((n) => (
                  <button
                    key={n.id}
                    className={view === n.id ? "active" : ""}
                    aria-current={view === n.id ? "page" : undefined}
                    onClick={() => go(n.id as View)}
                  >
                    {n.label}
                  </button>
                ))}
            </nav>
          )}
          {library ? (
            <>
              <div className="library-overview">
                <header className="library-header">
                  <div>
                    <div className="library-title-line">
                      <h1>{title}</h1>
                      <span
                        className="library-total"
                        aria-label={
                          r.loading
                            ? translate("불러오는 중")
                            : translate("{p0}개 아티팩트", {
                                p0: artifacts.length,
                              })
                        }
                      >
                        {r.loading ? "…" : artifacts.length}
                      </span>
                    </div>
                    <p>
                      {view === "archive"
                        ? translate(
                            "보관한 아티팩트는 자동 정리에서 제외합니다.",
                          )
                        : view === "mine"
                          ? translate(
                              "보고서부터 작은 웹앱까지, 내가 만든 아티팩트를 모아보세요.",
                            )
                          : view === "shared"
                            ? translate(
                                "조직 전체 또는 나에게 공유된 아티팩트를 볼 수 있습니다.",
                              )
                            : translate(
                                "문서, 웹앱, 프로토타입. 팀과 에이전트가 만든 결과물을 관리하세요.",
                              )}
                    </p>
                  </div>
                  <button
                    className="button primary"
                    onClick={() => setUpload(true)}
                  >
                    <Plus size={17} />
                    {translate("새 아티팩트")}
                  </button>
                </header>
                <div className="library-toolbar">
                  <nav
                    className="library-filters"
                    aria-label={translate("라이브러리 필터")}
                  >
                    {getNav()
                      .filter((n) => n.id !== "collections")
                      .map((n) => (
                        <button
                          key={n.id}
                          className={view === n.id ? "active" : ""}
                          aria-current={view === n.id ? "page" : undefined}
                          onClick={() => go(n.id as View)}
                        >
                          {n.label}
                        </button>
                      ))}
                  </nav>
                  <div className="toolbar-controls">
                    <div className="input-icon search-field">
                      <Search size={16} />
                      <input
                        ref={searchRef}
                        aria-label={translate("아티팩트 검색")}
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        placeholder={translate("아티팩트 검색…")}
                      />
                      <kbd>⌘K</kbd>
                    </div>
                    <Select
                      className="sort-select"
                      aria-label={translate("정렬")}
                      value={sort}
                      onValueChange={(value) => setSort(value)}
                    >
                      <option value="updated">
                        {translate("최근 수정순")}
                      </option>
                      <option value="title">{translate("제목순")}</option>
                    </Select>
                    <div
                      className="view-toggle"
                      aria-label={translate("보기 방식")}
                    >
                      <button
                        className={layout === "grid" ? "active" : ""}
                        aria-label={translate("그리드 보기")}
                        aria-pressed={layout === "grid"}
                        onClick={() => setLayout("grid")}
                      >
                        <Grid2X2 size={16} />
                      </button>
                      <button
                        className={layout === "list" ? "active" : ""}
                        aria-label={translate("목록 보기")}
                        aria-pressed={layout === "list"}
                        onClick={() => setLayout("list")}
                      >
                        <List size={16} />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
              <ErrorBox error={r.error} />
              {r.loading || r.stale ? (
                <div
                  className="artifact-grid"
                  aria-label={translate("불러오는 중")}
                >
                  {[0, 1, 2].map((i) => (
                    <div className="card-skeleton" key={i} />
                  ))}
                </div>
              ) : !artifacts.length ? (
                query ? (
                  <Empty
                    icon={<Search size={30} />}
                    heading={translate("검색 결과가 없습니다")}
                    description={translate(
                      "다른 제목이나 설명으로 검색해 보세요.",
                    )}
                    action={
                      <button
                        className="button secondary"
                        onClick={() => setQuery("")}
                      >
                        {translate("검색 지우기")}
                      </button>
                    }
                  />
                ) : (
                  <LibraryEmpty view={view} onUpload={() => setUpload(true)} />
                )
              ) : (
                <>
                  <div
                    className={`artifact-grid ${layout === "list" ? "list-layout" : ""}`}
                  >
                    {artifacts.map((artifact) => (
                      <ArtifactCard
                        key={artifact.id}
                        artifact={artifact}

                        onShare={() => setShare(artifact)}
                      />
                    ))}
                  </div>
                </>
              )}
            </>
          ) : view === "collections" ? (
            <CollectionsView
              user={session.user}
              onOpen={(artifact) => location.assign(artifact.shareUrl)}
              onShare={setShare}
            />
          ) : view === "trash" ? (
            <TrashView onChange={() => changed()} />
          ) : view === "account" ? (
            <AccountPage
              session={session}
              onSession={(next) => {
                onSession(next);
                setRevision((value) => value + 1);
              }}
            />
          ) : view === "agents" ? (
            <AgentsPage />
          ) : isAdmin ? (
            view === "backups" ? (
              <BackupSettings />
            ) : view === "members" ? (
              <MembersPage self={session.user} />
            ) : view === "storage" ? (
              <StoragePage />
            ) : view === "domains" ? (
              <DomainsPage />
            ) : view === "audit" ? (
              <AuditPage />
            ) : (
              <SettingsPage
                onChanged={() => {
                  api<Session>("/api/me").then((s) => {
                    setCSRF(s.csrfToken);
                    onSession(s);
                  });
                }}
              />
            )
          ) : (
            <Empty
              icon={<ShieldCheck size={30} />}
              heading={translate("관리자만 볼 수 있습니다")}
              description={translate("계정과 설치 설정은 관리자가 관리합니다.")}
            />
          )}
        </main>
      </div>
      {toast && (
        <div className="toast" role="status">
          <ShieldCheck size={17} />
          {translate(toast)}
          <button
            className="icon-button"
            aria-label={translate("알림 닫기")}
            onClick={() => setToast("")}
          >
            <X size={15} />
          </button>
        </div>
      )}
      {share && (
        <SharePanel
          artifact={share}
          canManage={canManage(share)}
          onClose={closeShare}
          onSaved={() => changed(translate("공유 설정을 저장했습니다."))}
        />
      )}
      {upload && (
        <UploadDialog
          existing={upload === true ? undefined : upload}
          onClose={closeUpload}
          onSaved={() => {
            changed(
              upload === true
                ? translate("새 아티팩트를 게시했습니다.")
                : translate("새 버전을 게시했습니다."),
            );
            go("mine");
          }}
        />
      )}
    </div>
  );
}
