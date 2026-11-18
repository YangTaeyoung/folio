import { useId, useRef, useState } from "react";
import { ArrowUpRight, Check, Copy, Eye, EyeOff } from "lucide-react";
import { api, copy, json } from "./api";
import { t, useLocale } from "./i18n";
import { ErrorBox, Field, Spinner } from "./ui";
import { Select } from "./Select";
import { ServiceIcon } from "./ServiceIcon";
import { connectionCommand } from "./agentClientCommands";
import type { AgentClient, GuideShell } from "./agentClientCommands";
import type { Token } from "./types";
import "./AgentClientGuides.css";

const clients: { id: AgentClient; name: string; docs: string }[] = [
  {
    id: "codex",
    name: "Codex",
    docs: "https://developers.openai.com/codex/mcp/",
  },
  {
    id: "claudecode",
    name: "Claude Code",
    docs: "https://code.claude.com/docs/en/mcp",
  },
  {
    id: "antigravity",
    name: "Antigravity CLI",
    docs: "https://antigravity.google/docs/mcp?tab=cli",
  },
  {
    id: "vscode",
    name: "VS Code · Copilot",
    docs: "https://code.visualstudio.com/docs/agents/reference/mcp-configuration",
  },
  { id: "cursor", name: "Cursor", docs: "https://cursor.com/docs/mcp" },
  {
    id: "opencode",
    name: "OpenCode",
    docs: "https://opencode.ai/docs/mcp-servers/",
  },
];

type IssuedConnection = { token: string; record: Token };

export function AgentClientGuides({
  origin = window.location.origin,
  onConnected,
}: {
  origin?: string;
  onConnected?: () => void;
}) {
  useLocale();
  const [selected, setSelected] = useState<AgentClient>("codex");
  const [shell, setShell] = useState<GuideShell>(() =>
    /Windows/i.test(navigator.userAgent) ? "powershell" : "posix",
  );
  const [issued, setIssued] = useState<
    Partial<Record<AgentClient, IssuedConnection>>
  >({});
  const [days, setDays] = useState("90");
  const [share, setShare] = useState(false);
  const [review, setReview] = useState(false);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  const [revealed, setRevealed] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [copyFailed, setCopyFailed] = useState(false);
  const inFlight = useRef(false);
  const id = useId();
  const client = clients.find((item) => item.id === selected)!;
  const current = issued[selected];
  const currentDays = current?.record.expiresAt
    ? String(
        Math.round(
          (new Date(current.record.expiresAt).getTime() -
            new Date(current.record.createdAt).getTime()) /
            86400000,
        ),
      )
    : days;
  const currentShare = current
    ? current.record.scopes.includes("share")
    : share;
  const currentReview = current
    ? current.record.scopes.includes("review")
    : review;
  const command = current
    ? connectionCommand(selected, origin, current.token, shell)
    : "";

  async function prepareAndCopy() {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError(null);
    setCopied(false);
    setCopyFailed(false);
    try {
      let connection = issued[selected];
      if (connection) {
        // A token revoked from the management list must not be recopied as usable.
        const { tokens } = await api<{ tokens: Token[] }>("/api/tokens");
        if (
          !tokens.some((item) => item.id === connection!.record.id) ||
          (connection.record.expiresAt &&
            new Date(connection.record.expiresAt).getTime() <= Date.now())
        ) {
          connection = undefined;
          setIssued((old) => ({ ...old, [selected]: undefined }));
        }
      }
      if (!connection) {
        connection = await api<IssuedConnection>(
          "/api/tokens",
          json("POST", {
            name: `Folio · ${client.name}`,
            scopes: [
              "read",
              "publish",
              ...(share ? ["share"] : []),
              ...(review ? ["review"] : []),
            ],
            expiresAt: new Date(
              Date.now() + Number(days) * 86400000,
            ).toISOString(),
          }),
        );
        setIssued((old) => ({ ...old, [selected]: connection }));
        onConnected?.();
      }
      try {
        await copy(
          connectionCommand(selected, origin, connection.token, shell),
        );
        setCopied(true);
      } catch {
        // Retain this exact issued token for retry or explicit manual selection.
        setCopyFailed(true);
      }
    } catch (error) {
      setError(error);
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }

  return (
    <section
      className="panel agent-client-guides"
      aria-labelledby={`${id}-title`}
    >
      <div className="section-title">
        <h2 id={`${id}-title`}>{t("사용할 에이전트 선택")}</h2>
      </div>
      <p className="subtle">
        {t(
          "사용할 에이전트를 고르고 명령 하나를 터미널에 붙여넣으세요. 토큰 발급과 연결 준비를 한 번에 처리합니다.",
        )}
      </p>
      <div
        className="agent-client-options"
        role="group"
        aria-label={t("연결할 에이전트")}
      >
        {clients.map((item) => (
          <button
            key={item.id}
            type="button"
            aria-pressed={selected === item.id}
            disabled={busy}
            onClick={() => {
              setSelected(item.id);
              setCopied(false);
              setCopyFailed(false);
              setRevealed(false);
              setError(null);
            }}
          >
            <ServiceIcon service={item.id} size={20} />
            <span>{item.name}</span>
            {selected === item.id && (
              <Check className="agent-client-check" size={14} />
            )}
          </button>
        ))}
      </div>
      <div className="agent-connection-settings">
        <div
          className="agent-shell-options"
          role="group"
          aria-label={t("터미널 환경")}
        >
          <button
            type="button"
            aria-pressed={shell === "posix"}
            disabled={busy}
            onClick={() => {
              setShell("posix");
              setCopied(false);
              setCopyFailed(false);
            }}
          >
            macOS / Linux
          </button>
          <button
            type="button"
            aria-pressed={shell === "powershell"}
            disabled={busy}
            onClick={() => {
              setShell("powershell");
              setCopied(false);
              setCopyFailed(false);
            }}
          >
            Windows · PowerShell
          </button>
        </div>
        <a
          className="agent-doc-link"
          href={client.docs}
          target="_blank"
          rel="noreferrer"
        >
          {t("공식 가이드")}
          <ArrowUpRight size={13} />
        </a>
      </div>
      <details
        className="agent-extra-guide agent-connection-options"
        onToggle={(event) => {
          if (!event.currentTarget.open || !current || busy) return;
          void api<{ tokens: Token[] }>("/api/tokens")
            .then(({ tokens }) => {
              if (
                !tokens.some((item) => item.id === current.record.id) ||
                (current.record.expiresAt &&
                  new Date(current.record.expiresAt).getTime() <= Date.now())
              ) {
                setIssued((old) => ({ ...old, [selected]: undefined }));
              }
            })
            .catch(setError);
        }}
      >
        <summary>
          {t("권한 및 만료 설정")}
          <span>
            {t("읽기·게시")} · {t("{count}일", { count: Number(currentDays) })}
          </span>
        </summary>
        <div className="agent-advanced-fields">
          <Field label={t("토큰 만료")}>
            <Select
              value={currentDays}
              disabled={busy || !!current}
              onValueChange={setDays}
            >
              <option value="30">{t("30일")}</option>
              <option value="90">{t("90일")}</option>
              <option value="365">{t("365일")}</option>
            </Select>
          </Field>
          <div className="agent-optional-scopes">
            <label>
              <input
                type="checkbox"
                checked={currentShare}
                disabled={busy || !!current}
                onChange={(event) => setShare(event.target.checked)}
              />
              {t("공유 권한 추가")}
            </label>
            <label>
              <input
                type="checkbox"
                checked={currentReview}
                disabled={busy || !!current}
                onChange={(event) => setReview(event.target.checked)}
              />
              {t("검토 권한 추가")}
            </label>
          </div>
        </div>
        {current && (
          <p className="agent-guide-hint">
            {t(
              "발급한 토큰은 아래 목록에서 관리할 수 있습니다. 새 권한이 필요하면 기존 토큰을 폐기한 뒤 다시 연결하세요.",
            )}
          </p>
        )}
      </details>
      <div className="agent-connection-action">
        <button
          type="button"
          className="button primary"
          disabled={busy}
          onClick={() => void prepareAndCopy()}
        >
          {busy ? (
            <Spinner />
          ) : copied ? (
            <Check size={16} />
          ) : (
            <Copy size={16} />
          )}
          {copied ? t("다시 복사") : t("연결 명령 복사")}
        </button>
        <span role="status" className="agent-copy-status">
          {copied
            ? t("복사했습니다. 터미널에 붙여넣어 연결하세요.")
            : current
              ? t("발급한 토큰으로 연결 명령을 복사합니다.")
              : t("클릭하면 이 에이전트용 토큰을 발급합니다.")}
        </span>
      </div>
      <ErrorBox error={error} />
      {copyFailed && (
        <p className="agent-copy-error" role="alert">
          {t(
            "복사하지 못했습니다. 다시 복사하거나 명령을 표시해 직접 복사하세요. 토큰은 다시 발급하지 않습니다.",
          )}
        </p>
      )}
      {current && (
        <details
          className="agent-extra-guide agent-command-disclosure"
          open={copyFailed || undefined}
        >
          <summary>{t("연결 명령 보기")}</summary>
          <div className="agent-command-toolbar">
            <span>{shell === "posix" ? "Bash / zsh" : "PowerShell"}</span>
            <button
              type="button"
              className="button ghost compact"
              aria-pressed={revealed}
              onClick={() => setRevealed(!revealed)}
            >
              {revealed ? <EyeOff size={14} /> : <Eye size={14} />}
              {revealed ? t("토큰 숨기기") : t("토큰 표시")}
            </button>
          </div>
          <textarea
            className="agent-command-text"
            readOnly
            rows={5}
            spellCheck={false}
            aria-label={t("연결 명령")}
            value={
              revealed
                ? command
                : connectionCommand(selected, origin, "••••••••", shell)
            }
            onFocus={(event) => {
              if (revealed) event.target.select();
            }}
          />
          <p className="agent-guide-hint">
            {t(
              "명령에는 비밀 토큰이 포함됩니다. 공유하지 마세요. 토큰은 에이전트 설정에 넣지 않고 Folio CLI의 사용자 설정에 저장합니다.",
            )}
          </p>
        </details>
      )}
      <p className="agent-guide-hint">
        {t(
          "Folio CLI와 선택한 에이전트가 설치되어 있어야 합니다. 명령 실행 후 에이전트를 다시 열고 “folio_capabilities 도구로 연결을 확인해 줘”라고 요청하세요.",
        )}
      </p>
      {selected === "vscode" && (
        <p className="agent-guide-hint">
          {t(
            "VS Code 사용자 프로필에 등록합니다. 명령 팔레트의 MCP: List Servers에서 Folio를 시작하고 도구 연결을 확인하세요.",
          )}
        </p>
      )}
      {(selected === "codex" || selected === "claudecode") && (
        <details className="agent-extra-guide">
          <summary>{t("게시·검토 워크플로 스킬 추가 (선택)")}</summary>
          <code className="agent-skill-command">
            folio skills install --client{" "}
            {selected === "codex" ? "codex" : "claude"}
          </code>
          <p className="agent-guide-hint">
            {t(
              "사용자 스킬 폴더에 Folio 안내를 설치합니다. 기존 Folio 스킬은 덮어쓰지 않습니다.",
            )}
          </p>
        </details>
      )}
    </section>
  );
}
