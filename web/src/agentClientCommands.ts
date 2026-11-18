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
  shell: GuideShell,
): string {
  switch (client) {
    case "codex":
      return "codex mcp add folio -- folio mcp";
    case "claudecode":
      return "claude mcp add --scope user --transport stdio folio -- folio mcp";
    case "antigravity":
      return "agy mcp add --type stdio folio folio mcp";
    case "vscode": {
      const configuration = JSON.stringify({
        name: "folio",
        type: "stdio",
        command: "folio",
        args: ["mcp"],
      });
      return shell === "powershell"
        ? `code --% --add-mcp ${configuration.replaceAll('"', '\\"')}`
        : `code --add-mcp ${quotePosix(configuration)}`;
    }
    case "cursor":
      return "folio connect --client cursor";
    case "opencode":
      return "opencode mcp add folio -- folio mcp";
  }
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
  const registration = registrationCommand(client, shell);
  const verification = verificationCommand(client);
  if (shell === "powershell") {
    const commands = [
      `    $folioToken | folio login --url ${quotePowerShell(origin)} --token-stdin`,
      "    if ($LASTEXITCODE -ne 0) { throw 'Folio login failed.' }",
      `    ${registration}`,
      "    if ($LASTEXITCODE -ne 0) { throw 'MCP registration failed.' }",
      "    folio doctor",
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
      "  Get-Command folio -ErrorAction Stop | Out-Null",
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
    "command -v folio >/dev/null",
    ...(executable !== "folio" ? [`command -v ${executable} >/dev/null`] : []),
    `printf '%s' ${quotePosix(token)} | folio login --url ${quotePosix(origin)} --token-stdin`,
    registration,
    "folio doctor",
    ...verification.split("\n").filter(Boolean),
  ].join("\n");
  let delimiter = "FOLIO_CONNECT";
  while (script.split("\n").includes(delimiter)) delimiter += "_";
  return `bash <<'${delimiter}'\n${script}\n${delimiter}`;
}
