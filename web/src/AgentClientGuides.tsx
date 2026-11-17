import { useId, useState } from "react";
import { ArrowUpRight, Check, Terminal } from "lucide-react";
import { t, useLocale } from "./i18n";
import { CopyButton } from "./ui";
import { ServiceIcon } from "./ServiceIcon";
import {
  cursorConfiguration,
  loginCommand,
  registrationCommand,
  verificationCommand,
} from "./agentClientCommands";
import type { AgentClient, GuideShell } from "./agentClientCommands";
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
    id: "geminicli",
    name: "Gemini CLI",
    docs: "https://geminicli.com/docs/tools/mcp-server/",
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

function Command({
  text,
  label,
  language = "terminal",
}: {
  text: string;
  label: string;
  language?: string;
}) {
  return (
    <div className="agent-command">
      <div className="agent-command-toolbar">
        <span>{language}</span>
        <CopyButton text={text} label={label} />
      </div>
      <pre tabIndex={0}>
        <code>{text}</code>
      </pre>
    </div>
  );
}

export function AgentClientGuides({
  origin = window.location.origin,
}: {
  origin?: string;
}) {
  useLocale();
  const [selected, setSelected] = useState<AgentClient>("codex");
  const [shell, setShell] = useState<GuideShell>("posix");
  const id = useId();
  const client = clients.find((item) => item.id === selected)!;
  return (
    <section
      className="panel agent-client-guides"
      aria-labelledby={`${id}-title`}
    >
      <div className="section-title">
        <h2 id={`${id}-title`}>{t("2. 에이전트 연결")}</h2>
        <ServiceIcon service="mcp" size={21} />
      </div>
      <p className="subtle">
        {t(
          "Folio CLI와 에이전트를 같은 컴퓨터에 설치한 뒤, 아래 순서로 연결하세요.",
        )}
      </p>
      <div
        className="agent-shell-options"
        role="group"
        aria-label={t("터미널 환경")}
      >
        <button
          type="button"
          aria-pressed={shell === "posix"}
          onClick={() => setShell("posix")}
        >
          macOS / Linux
        </button>
        <button
          type="button"
          aria-pressed={shell === "powershell"}
          onClick={() => setShell("powershell")}
        >
          Windows · PowerShell
        </button>
      </div>

      <div className="agent-setup-step">
        <div className="agent-step-heading">
          <span className="agent-step-number">A</span>
          <h3>{t("먼저 CLI에 로그인")}</h3>
        </div>
        <p className="subtle">
          {t(
            "위에서 만든 API 토큰을 터미널의 비공개 입력란에 붙여넣으세요. 로그인은 이 컴퓨터에서 한 번만 하면 됩니다.",
          )}
        </p>
        <Command
          text={loginCommand(origin, shell)}
          label={t("로그인 명령 복사")}
          language={shell === "posix" ? "Bash / zsh" : "PowerShell"}
        />
        <p className="agent-guide-hint">
          {t(
            "토큰은 Folio CLI의 사용자 설정에 저장됩니다. 아래 에이전트 설정에는 토큰을 넣지 않습니다.",
          )}
        </p>
      </div>

      <div className="agent-setup-step">
        <div className="agent-step-heading">
          <span className="agent-step-number">B</span>
          <h3>{t("사용할 에이전트 선택")}</h3>
        </div>
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
              onClick={() => setSelected(item.id)}
            >
              <ServiceIcon service={item.id} size={21} />
              <span>{item.name}</span>
              {selected === item.id && (
                <Check className="agent-client-check" size={15} />
              )}
            </button>
          ))}
        </div>
        <div className="agent-client-instructions" key={selected}>
          <div className="agent-client-heading">
            <h4>{t("{agent}에 등록", { agent: client.name })}</h4>
            <a href={client.docs} target="_blank" rel="noreferrer">
              {t("공식 가이드")}
              <ArrowUpRight size={14} />
            </a>
          </div>
          <Command
            text={registrationCommand(selected, shell)}
            label={t("등록 명령 복사")}
          />
          {selected === "cursor" && (
            <>
              <p className="agent-guide-hint">
                {t(
                  "cursor 명령은 설정 파일을 에디터에서 엽니다. 열린 mcp.json의 mcpServers에 folio 항목을 추가하고 저장하세요. 기존 서버 설정은 유지하세요.",
                )}
              </p>
              <Command
                text={cursorConfiguration}
                label={t("Cursor 설정 복사")}
                language="mcp.json"
              />
            </>
          )}
          {selected === "opencode" && (
            <p className="agent-guide-hint">
              {t(
                "설정 범위를 직접 고르려면 opencode mcp add를 실행하세요. 대화형 설정에서 프로젝트 또는 사용자를 고른 뒤 이름은 folio, 유형은 Local, 실행 명령은 folio mcp를 입력하세요.",
              )}
            </p>
          )}
          {selected === "vscode" && (
            <p className="agent-guide-hint">
              {t(
                "VS Code 사용자 프로필에 등록합니다. macOS에서 code 명령이 없으면 명령 팔레트에서 Shell Command: Install 'code' command in PATH를 실행하세요.",
              )}
            </p>
          )}
          {(selected === "codex" ||
            selected === "claudecode" ||
            selected === "geminicli") && (
            <p className="agent-guide-hint">
              {t(
                "사용자 범위에 등록되어 여러 프로젝트에서 사용할 수 있습니다. 등록 후 에이전트 세션을 다시 여세요.",
              )}
            </p>
          )}
          <h4 className="agent-verify-heading">
            <Terminal size={15} />
            {t("연결 확인")}
          </h4>
          {selected === "vscode" && (
            <p className="agent-guide-hint">
              {t(
                "VS Code 명령 팔레트에서 아래 명령을 실행하고 folio를 선택한 뒤 서버를 시작하세요. 서버 신뢰 확인이 표시되면 내용을 확인하세요.",
              )}
            </p>
          )}
          <Command
            text={verificationCommand(selected)}
            label={t("확인 명령 복사")}
            language={selected === "vscode" ? t("명령 팔레트") : "terminal"}
          />
          <p className="agent-guide-hint">
            {t(
              "에이전트에 “folio_capabilities 도구로 연결을 확인해 줘”라고 요청하세요. 설치 정보와 한도가 반환되면 사용할 준비가 됐습니다.",
            )}
          </p>
          {(selected === "codex" || selected === "claudecode") && (
            <details className="agent-extra-guide">
              <summary>{t("게시·검토 워크플로 스킬 추가 (선택)")}</summary>
              <Command
                text={`folio skills install --client ${selected === "codex" ? "codex" : "claude"}`}
                label={t("스킬 설치 명령 복사")}
              />
              <p className="agent-guide-hint">
                {t(
                  "사용자 스킬 폴더에 Folio 안내를 설치합니다. 기존 Folio 스킬은 덮어쓰지 않습니다.",
                )}
              </p>
            </details>
          )}
        </div>
      </div>
      <details className="agent-extra-guide">
        <summary>{t("명령을 찾지 못하거나 연결이 안 되나요?")}</summary>
        <p className="subtle">
          {t(
            "folio doctor로 URL과 인증을 확인하세요. folio 명령을 찾지 못하면 실행 파일을 PATH에 추가하거나 에이전트 설정의 command를 Folio 실행 파일의 절대 경로로 바꾸세요.",
          )}
        </p>
        <Command text="folio doctor" label={t("진단 명령 복사")} />
        <p className="agent-guide-hint">
          {t(
            "에이전트와 CLI는 같은 OS 사용자와 Folio 설정 파일을 사용해야 합니다. 원격 개발·컨테이너에서는 MCP 실행 환경에서 로그인하거나 FOLIO_CONFIG로 설정 파일을 지정하세요. FOLIO_URL·FOLIO_TOKEN 환경변수는 저장된 로그인보다 우선합니다.",
          )}
        </p>
      </details>
    </section>
  );
}
