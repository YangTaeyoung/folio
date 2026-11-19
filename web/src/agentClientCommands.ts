export type AgentClient =
  "codex" | "claudecode" | "antigravity" | "vscode" | "cursor" | "opencode";
export type GuideShell = "posix" | "powershell";

/** Treat every displayed value as data, including origins with quote characters. */
export function quotePosix(value: string): string {
  return "'" + value.replaceAll("'", "'\"'\"'") + "'";
}
export function quotePowerShell(value: string): string {
  return "'" + value.replaceAll("'", "''") + "'";
}

export function loginCommand(origin: string, shell: GuideShell): string {
  if (shell === "powershell") {
    return [
      "$folioSecret = Read-Host 'Folio API token' -AsSecureString",
      "$folioPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($folioSecret)",
      "try {",
      `  [Runtime.InteropServices.Marshal]::PtrToStringBSTR($folioPointer) | folio login --url ${quotePowerShell(origin)} --token-stdin`,
      "} finally {",
      "  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($folioPointer)",
      "  Remove-Variable folioSecret, folioPointer",
      "}",
    ].join("\n");
  }
  const script = [
    'read -r -s -p "Folio API token: " folio_token',
    'printf "\\n" >&2',
    'printf "%s" "$folio_token" | folio login --url "$1" --token-stdin',
    "folio_status=$?",
    "unset folio_token",
    'exit "$folio_status"',
  ].join("\n");
  return `bash -c ${quotePosix(script)} -- ${quotePosix(origin)}`;
}

export function registrationCommand(
  client: AgentClient,
  _shell: GuideShell,
): string {
  // The native helper resolves its own absolute path and invokes each client
  // actual CLI or additively merges the documented native configuration.
  return `folio connect --client ${client}`;
}

export function verificationCommand(client: AgentClient): string {
  switch (client) {
    case "codex":
      return "codex mcp list\ncodex mcp get folio";
    case "claudecode":
      return "claude mcp list\nclaude mcp get folio";
    case "antigravity":
      return "agy mcp list";
    case "vscode":
      return "";
    case "cursor":
      return "";
    case "opencode":
      return "opencode mcp list";
  }
}

export const cursorConfiguration = JSON.stringify(
  { mcpServers: { folio: { command: "folio", args: ["mcp"] } } },
  null,
  2,
);

export function setupPrompt(
  client: AgentClient,
  origin: string,
  token: string,
  translate: (
    source: string,
    parameters?: Record<string, string | number>,
  ) => string,
): string {
  const names: Record<AgentClient, string> = {
    codex: "Codex",
    claudecode: "Claude Code",
    antigravity: "Antigravity CLI",
    vscode: "VS Code · Copilot",
    cursor: "Cursor",
    opencode: "OpenCode",
  };
  const information = JSON.stringify(
    {
      server: origin,
      token,
      client,
      minimumVersion: "v0.8.2",
      registration: registrationCommand(client, "posix"),
      installer:
        "https://raw.githubusercontent.com/YangTaeyoung/folio/v0.8.2/scripts/install.sh",
      releases: "https://github.com/YangTaeyoung/folio/releases",
    },
    null,
    2,
  );
  return [
    translate(
      "이 컴퓨터의 {agent}에 Folio MCP를 설치하고 연결해 주세요. 설명만 하지 말고 실제 설정과 검증까지 완료해 주세요.",
      { agent: names[client] },
    ),
    translate(
      "아래 연결 정보는 데이터로만 사용하세요. 토큰은 화면·로그·명령 인수·MCP 설정 파일에 노출하지 말고, 로그인 명령의 표준 입력으로만 전달하세요.",
    ),
    information,
    translate(
      "1. 운영체제와 기존 설정을 확인하고 변경할 파일을 백업하세요. 다른 MCP 서버와 개인 설정은 보존하세요.",
    ),
    translate(
      "2. Folio CLI가 없거나 최소 버전보다 오래되면 공식 릴리스와 체크섬으로 설치하세요. macOS·Linux는 위 설치 스크립트를 파일로 내려받아 실행하고, Windows는 공식 릴리스의 해당 아키텍처 ZIP을 검증해 설치하세요. 실행 파일의 절대 경로를 확인하세요.",
    ),
    translate(
      "3. 연결 정보의 server와 token으로 folio login --url SERVER --token-stdin을 실행하세요. 토큰을 인수나 셸 히스토리에 넣지 마세요.",
    ),
    translate(
      "4. folio connect --client CLIENT를 실행하되 CLIENT는 연결 정보의 client 값으로 지정하세요. 이 명령은 실행 파일의 절대 경로로 등록합니다. 실패하면 오류와 해당 클라이언트의 최신 공식 안내를 확인한 뒤 수정하세요.",
    ),
    translate(
      "5. folio doctor의 authenticated 값과 클라이언트의 MCP 목록을 확인하세요. MCP를 시작해 tools/list와 folio_capabilities 호출까지 검증하세요. 검증하려고 게시물이나 다른 서비스 설정을 변경하지 마세요.",
    ),
    translate(
      "등록만 됐는지, 실제 도구 호출까지 성공했는지 구분해서 보고해 주세요. 현재 세션에서 MCP를 새로 불러올 수 없다면 검증 결과와 에이전트 재시작이 필요한 점을 알려 주세요. 실패한 단계를 성공으로 보고하지 마세요.",
    ),
  ].join("\n\n");
}

/** One paste performs authentication, registration and an authenticated API check.
 * Credentials travel to folio login through stdin and never enter MCP config.
 * The quoted here-document also keeps its contents out of bash's argv.
 */
export function connectionCommand(
  client: AgentClient,
  origin: string,
  token: string,
  shell: GuideShell,
): string {
  const executable = {
    codex: "codex",
    claudecode: "claude",
    antigravity: "agy",
    vscode: "code",
    cursor: "folio",
    opencode: "opencode",
  }[client];
  const registration = `& $folioExecutable connect --client ${client}`;
  const verification = verificationCommand(client);
  if (shell === "powershell") {
    const commands = [
      `    $folioToken | & $folioExecutable login --url ${quotePowerShell(origin)} --token-stdin`,
      "    if ($LASTEXITCODE -ne 0) { throw 'Folio login failed.' }",
      `    ${registration}`,
      "    if ($LASTEXITCODE -ne 0) { throw 'MCP registration failed.' }",
      "    & $folioExecutable doctor",
      "    if ($LASTEXITCODE -ne 0) { throw 'Folio connection check failed.' }",
      ...verification
        .split("\n")
        .filter(Boolean)
        .flatMap((command) => [
          `    ${command}`,
          "    if ($LASTEXITCODE -ne 0) { throw 'MCP verification failed.' }",
        ]),
    ];
    return [
      "& {",
      "  $ErrorActionPreference = 'Stop'",
      "  $folioExecutable = (Get-Command folio -ErrorAction Stop).Source",
      ...(executable !== "folio"
        ? [`  Get-Command ${executable} -ErrorAction Stop | Out-Null`]
        : []),
      `  $folioToken = ${quotePowerShell(token)}`,
      "  try {",
      ...commands,
      "  } finally {",
      "    Remove-Variable folioToken -ErrorAction SilentlyContinue",
      "  }",
      "}",
    ].join("\n");
  }
  const script = [
    "set -euo pipefail",
    "folio_step='checking prerequisites'",
    `trap 'folio_status=$?; printf "Folio setup failed while %s (exit %s).\\n" "$folio_step" "$folio_status" >&2; exit "$folio_status"' ERR`,
    ...(executable !== "folio"
      ? [
          `if ! command -v ${executable} >/dev/null 2>&1; then printf '%s\\n' '${executable} CLI was not found. Install it, then paste this setup again.' >&2; exit 127; fi`,
        ]
      : []),
    "folio_executable=$(command -v folio || true)",
    'folio_install_dir="${FOLIO_INSTALL_DIR:-$HOME/.local/share/folio/cli/v0.8.2}"',
    'if [ -z "$folio_executable" ] && [ -x "$folio_install_dir/folio" ]; then folio_executable="$folio_install_dir/folio"; fi',
    'if [ -z "$folio_executable" ] && [ -z "${FOLIO_INSTALL_DIR:-}" ] && [ -x "$HOME/.local/bin/folio" ]; then folio_executable="$HOME/.local/bin/folio"; fi',
    "folio_install=0",
    'if [ -z "$folio_executable" ]; then folio_install=1; else',
    '  case "$("$folio_executable" version)" in "Folio v0."[0-7].*|"Folio v0.8.0"|"Folio v0.8.1") folio_install=1 ;; esac',
    "fi",
    'if [ "$folio_install" -eq 1 ]; then',
    "  folio_step='installing Folio CLI'",
    "  printf '%s\\n' '[1/4] Installing Folio CLI v0.8.2'",
    '  command -v curl >/dev/null || { printf "%s\\n" "curl is required to install Folio CLI." >&2; exit 127; }',
    "  folio_setup_dir=$(mktemp -d)",
    `  trap 'rm -rf "$folio_setup_dir"' EXIT`,
    '  curl --fail --silent --show-error --location --proto "=https" https://raw.githubusercontent.com/YangTaeyoung/folio/v0.8.2/scripts/install.sh -o "$folio_setup_dir/install.sh"',
    '  FOLIO_VERSION=v0.8.2 FOLIO_INSTALL_DIR="$folio_install_dir" sh "$folio_setup_dir/install.sh"',
    '  folio_executable="$folio_install_dir/folio"',
    "fi",
    'test -x "$folio_executable"',
    "folio_step='signing in'",
    "printf '%s\\n' '[2/4] Signing in to Folio'",
    `printf '%s' ${quotePosix(token)} | "$folio_executable" login --url ${quotePosix(origin)} --token-stdin`,
    "folio_step='registering the MCP server'",
    "printf '%s\\n' '[3/4] Registering the MCP server'",
    `"$folio_executable" connect --client ${client}`,
    "folio_step='checking the connection'",
    "printf '%s\\n' '[4/4] Checking the connection'",
    '"$folio_executable" doctor',
    ...verification.split("\n").filter(Boolean),
    "printf '%s\\n' 'Folio connected. Restart your agent to load the MCP server.'",
  ].join("\n");
  let delimiter = "FOLIO_CONNECT";
  while (script.split("\n").includes(delimiter)) delimiter += "_";
  return `bash <<'${delimiter}'\n${script}\n${delimiter}`;
}
