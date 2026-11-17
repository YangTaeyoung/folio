export type AgentClient =
  "codex" | "claudecode" | "geminicli" | "vscode" | "cursor" | "opencode";
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
    case "geminicli":
      return "gemini mcp add --scope user --transport stdio folio folio mcp";
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
      return shell === "powershell"
        ? 'New-Item -ItemType Directory -Force "$HOME/.cursor" | Out-Null\ncursor "$HOME/.cursor/mcp.json"'
        : 'mkdir -p "$HOME/.cursor"\ncursor "$HOME/.cursor/mcp.json"';
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
    case "geminicli":
      return "gemini mcp list";
    case "vscode":
      return "MCP: List Servers";
    case "cursor":
      return "agent mcp list\nagent mcp list-tools folio";
    case "opencode":
      return "opencode mcp list";
  }
}

export const cursorConfiguration = JSON.stringify(
  { mcpServers: { folio: { command: "folio", args: ["mcp"] } } },
  null,
  2,
);
