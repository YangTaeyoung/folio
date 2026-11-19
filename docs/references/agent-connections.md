# Agent quick connection

Checked 2026-11-19 against current official documentation, installed CLI help, and project source. Connection UI: `web/src/AgentClientGuides.tsx`; shell generation: `web/src/agentClientCommands.ts`; additive Cursor registration: `cmd/folio/connect.go`.

## One explicit action

Selecting a client does not issue credentials. **Copy connection command** issues `/api/tokens` with an automatic client-specific name, read/publish scopes, and a 90-day expiry. Share/review permissions and expiry are adjustable in a small disclosure before issuance. The token API returns `{token, record}` (`internal/server/auth.go`, `tokens`).

The alternative terminal script performs preflight executable checks, `folio login --url URL --token-stdin`, registration, `folio doctor`, and available client list/get verification. It stops at a failed command. Token data goes through stdin to `folio login`; the client registration contains only a local `folio mcp` launcher. The script itself contains the secret and should be treated as a credential, including terminal history and clipboard contents.

Issued credentials stay only in component memory. The displayed command is masked until explicitly revealed. Clipboard failure retains the exact command for retry or manual selection and does not issue a second token. Recopy checks `/api/tokens`; a revoked or expired token is replaced only on the next explicit copy action. The token management list remains the place to revoke credentials.

Folio login validates `/api/me` and saves the local CLI config (`cmd/folio/main.go`). `FOLIO_CONFIG` selects a different file; `FOLIO_URL` and `FOLIO_TOKEN` override saved values (`internal/agent/client.go`). POSIX config directory/file creation modes are 0700/0600; this does not establish Windows ACL behavior.

## Registration evidence

| Client | Setup command | Native registration |
| --- | --- | --- |
| Codex | `folio connect --client codex` | `codex mcp add folio -- <absolute-folio> mcp` |
| Claude Code | `folio connect --client claudecode` | `claude mcp add --scope user --transport stdio folio -- <absolute-folio> mcp` |
| Antigravity CLI | `folio connect --client antigravity` | `agy mcp add --type stdio folio <absolute-folio> mcp` |
| VS Code / Copilot | `folio connect --client vscode` | Native `code --add-mcp` with generated JSON containing an absolute executable path |
| Cursor | `folio connect --client cursor` | Additive `~/.cursor/mcp.json` update |
| OpenCode | `folio connect --client opencode` | Additive native `mcp` configuration with a local command array |

Antigravity replaces Gemini CLI in the primary chooser. Antigravity MCP reference documents CLI stdio support and its interactive manager; the exact noninteractive add syntax was checked in installed help.

Codex, Claude Code, Antigravity, and OpenCode scripts run their documented MCP list command; Codex and Claude also run get. Cursor verifies its merge and Folio's authenticated API rather than requiring the separately installed `agent` executable. VS Code has no verified terminal list command, so the UI directs users to **MCP: List Servers** to start/check the server. Every client still needs an actual `folio_capabilities` tool call to prove its authenticated MCP runtime.

**VS Code compatibility boundary:** current official documentation says `code --add-mcp` targets editor-managed user-profile configuration. It does not write portable Copilot `mcp-config.json`. The Local harness is deprecated and scheduled for removal in February 2027; current Copilot native configuration uses `mcpServers` in global `mcp-config.json` or workspace-root `.mcp.json`. The copied command uses the still-documented CLI and the guide names its profile destination. A native Copilot destination requires additional verified configuration support rather than an invented CLI flag.

**Windows:** PowerShell generation uses a child script scope, literal single-quote escaping, stdin delivery, explicit `$LASTEXITCODE` checks, and a `finally` block to remove its temporary token variable. VS Code native JSON is generated and passed as a single argument by the Go connection helper, avoiding PowerShell JSON re-parsing. [Microsoft parsing reference](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_parsing). Windows runtime execution remains unverified.

## Safe Cursor merge

`folio connect --client cursor` configures only the local token-free launcher. `--config-dir DIRECTORY` or `FOLIO_CONFIG_DIR` changes the Cursor directory for isolation; neither changes Folio's separate credential path. Existing unknown settings and MCP servers are preserved. Exact existing launcher configuration is a no-op; malformed JSON, a non-object server map, a different existing `folio` entry, and non-regular files are refused without changing the destination. Writes use a same-directory temporary file with 0600 mode and final rename. The command checks for destination changes before replacing it.

Go tests use temporary directories, never the user's actual Cursor config. They verify preservation of unrelated server settings, idempotence, refusal without modification, explicit isolated paths, and symlinks.

## Validation scope

`node --experimental-strip-types --test web/tests/agent_commands.test.ts` exercises the generated Bash script through both Bash and zsh using disposable native-command stubs, including literal hostile data, stdin delivery, no credential in native executable arguments/stdout, registration order, and stop-on-failure behavior. PowerShell generation is checked structurally. This does not prove registration in every third-party client or Windows execution.

Optional Codex/Claude publishing skills remain available as `folio skills install --client codex|claude`. Skills supplement MCP registration and do not authenticate the CLI.

The integrated `dev-v08` binary passed `web/tests/agent_guides.py` on 2026-11-19: all six clients, light/dark themes, widths 1440/390/320, token-inclusive clipboard flow, token reuse on recopy/shell change, revoked-token renewal with 30-day share/review opt-ins, keyboard selection, and all five locales. All 36 axe color-contrast states passed with no browser page errors. Evidence is in `.work/ux-v08/agent-guides/`; captures contain authored fixture data and masked credentials. The separate disposable quick-connect smoke also forced clipboard denial twice and confirmed no additional token issuance.

## Setup prompt and verified native registration (2026-11-20)

The primary copy action now provides a localized setup prompt, including scoped connection data, official installation sources, the minimum CLI version, and explicit registration/API/MCP verification steps. The terminal script remains an alternative. Both reuse the same token; switching delivery methods does not issue a token. Hidden previews mask credentials, including the prompt's JSON data.

`folio connect --client CLIENT` resolves its absolute executable path and registers Codex, Claude Code, Antigravity CLI and VS Code through their native CLIs. Cursor and OpenCode receive additive, token-free native configuration. Other servers and unknown settings are preserved; conflicting existing Folio entries and malformed JSON are refused. OpenCode uses its documented `mcp` / `type: local` / `command: [...]` format; its interactive `mcp add` is no longer presented as a noninteractive shell command. References: [OpenCode MCP](https://opencode.ai/docs/mcp-servers/), [OpenCode config](https://opencode.ai/docs/config/), [Codex MCP](https://developers.openai.com/codex/mcp/), [Claude configuration](https://code.claude.com/docs/en/settings), [VS Code MCP](https://code.visualstudio.com/docs/agents/reference/mcp-configuration), [Antigravity MCP](https://antigravity.google/docs/mcp?tab=cli). Installed CLI help and actual registrations were also inspected.

The Bash script reports progress and errors without echoing the failing command or credentials. It detects a private installation outside PATH, and downloads the release's checksum-verifying installer when missing or below v0.8.2. New downloads use a versioned private directory, so an older binary used by another service is not overwritten. Existing client CLI prerequisites are checked before authentication. Downloads fail before login or registration.

Actual macOS checks used Codex, Claude Code, Antigravity CLI, official VS Code 1.147.0, OpenCode 1.19.6, and the real Folio Cursor writer. Codex initialization, tools/list (23 tools), and authenticated folio_capabilities passed. Claude, Cursor, OpenCode and VS Code used disposable settings; OpenCode's actual MCP list reported connected. Antigravity ignored the attempted directory override, so its temporary registration was removed and its original empty list restored. The CLI now rejects unsupported Antigravity isolation before mutation. PowerShell 7.7.0 was downloaded with published checksum verification for syntax/execution checks on macOS; this is not evidence of a Windows runtime check.
