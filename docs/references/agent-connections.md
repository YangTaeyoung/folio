# Agent quick connection

Checked 2026-11-19 against current official documentation, installed CLI help, and project source. Connection UI: `web/src/AgentClientGuides.tsx`; shell generation: `web/src/agentClientCommands.ts`; additive Cursor registration: `cmd/folio/connect.go`.

## One explicit action

Selecting a client does not issue credentials. **Copy connection command** issues `/api/tokens` with an automatic client-specific name, read/publish scopes, and a 90-day expiry. Share/review permissions and expiry are adjustable in a small disclosure before issuance. The token API returns `{token, record}` (`internal/server/auth.go`, `tokens`).

The copied script performs preflight executable checks, `folio login --url URL --token-stdin`, registration, `folio doctor`, and available client list/get verification. It stops at a failed command. Token data goes through stdin to `folio login`; the client registration contains only a local `folio mcp` launcher. The script itself contains the secret and should be treated as a credential, including terminal history and clipboard contents.

Issued credentials stay only in component memory. The displayed command is masked until explicitly revealed. Clipboard failure retains the exact command for retry or manual selection and does not issue a second token. Recopy checks `/api/tokens`; a revoked or expired token is replaced only on the next explicit copy action. The token management list remains the place to revoke credentials.

Folio login validates `/api/me` and saves the local CLI config (`cmd/folio/main.go`). `FOLIO_CONFIG` selects a different file; `FOLIO_URL` and `FOLIO_TOKEN` override saved values (`internal/agent/client.go`). POSIX config directory/file creation modes are 0700/0600; this does not establish Windows ACL behavior.

## Registration evidence

| Client            | Registration in the copied script                                                   | Evidence checked                                                                                                                                                                                                                           |
| ----------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Codex             | `codex mcp add folio -- folio mcp`                                                  | [Official MCP guide](https://developers.openai.com/codex/mcp/) and installed `codex mcp add/get --help`                                                                                                                                    |
| Claude Code       | `claude mcp add --scope user --transport stdio folio -- folio mcp`                  | [Official MCP guide](https://code.claude.com/docs/en/mcp) and installed `claude mcp add/get --help`                                                                                                                                        |
| Antigravity CLI   | `agy mcp add --type stdio folio folio mcp`                                          | [Official MCP guide](https://antigravity.google/docs/mcp?tab=cli) and installed `agy mcp add/list --help` (1.5.11). Flags precede the server name; this CLI has no scope flag in its add help.                                             |
| VS Code / Copilot | `code --add-mcp '{"name":"folio","type":"stdio","command":"folio","args":["mcp"]}'` | [Official MCP configuration reference](https://code.visualstudio.com/docs/agents/reference/mcp-configuration), command-line installation section                                                                                           |
| Cursor            | `folio connect --client cursor`                                                     | [Official configuration schema and global path](https://cursor.com/docs/mcp); project implementation merges a token-free launcher into `~/.cursor/mcp.json`                                                                                |
| OpenCode          | `opencode mcp add folio -- folio mcp`                                               | [Official MCP guide](https://opencode.ai/docs/mcp-servers/) and [v1.19.6 CLI source](https://github.com/anomalyco/opencode/blob/v1.19.6/packages/opencode/src/cli/cmd/mcp.ts), `McpAddCommand` accepts the name and command following `--` |

Antigravity replaces Gemini CLI in the primary chooser. Antigravity MCP reference documents CLI stdio support and its interactive manager; the exact noninteractive add syntax was checked in installed help.

Codex, Claude Code, Antigravity, and OpenCode scripts run their documented MCP list command; Codex and Claude also run get. Cursor verifies its merge and Folio's authenticated API rather than requiring the separately installed `agent` executable. VS Code has no verified terminal list command, so the UI directs users to **MCP: List Servers** to start/check the server. Every client still needs an actual `folio_capabilities` tool call to prove its authenticated MCP runtime.

**VS Code compatibility boundary:** current official documentation says `code --add-mcp` targets editor-managed user-profile configuration. It does not write portable Copilot `mcp-config.json`. The Local harness is deprecated and scheduled for removal in February 2027; current Copilot native configuration uses `mcpServers` in global `mcp-config.json` or workspace-root `.mcp.json`. The copied command uses the still-documented CLI and the guide names its profile destination. A native Copilot destination requires additional verified configuration support rather than an invented CLI flag.

**Windows:** PowerShell generation uses a child script scope, literal single-quote escaping, stdin delivery, explicit `$LASTEXITCODE` checks, and a `finally` block to remove its temporary token variable. VS Code native JSON uses the documented `--%` stop-parsing form. [Microsoft parsing reference](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_parsing). Windows runtime execution remains unverified.

## Safe Cursor merge

`folio connect --client cursor` configures only the local token-free launcher. `--config-dir DIRECTORY` or `FOLIO_CONFIG_DIR` changes the Cursor directory for isolation; neither changes Folio's separate credential path. Existing unknown settings and MCP servers are preserved. Exact existing launcher configuration is a no-op; malformed JSON, a non-object server map, a different existing `folio` entry, and non-regular files are refused without changing the destination. Writes use a same-directory temporary file with 0600 mode and final rename. The command checks for destination changes before replacing it.

Go tests use temporary directories, never the user's actual Cursor config. They verify preservation of unrelated server settings, idempotence, refusal without modification, explicit isolated paths, and symlinks.

## Validation scope

`node --experimental-strip-types --test web/tests/agent_commands.test.ts` exercises the generated Bash script through both Bash and zsh using disposable native-command stubs, including literal hostile data, stdin delivery, no credential in native executable arguments/stdout, registration order, and stop-on-failure behavior. PowerShell generation is checked structurally. This does not prove registration in every third-party client or Windows execution.

Optional Codex/Claude publishing skills remain available as `folio skills install --client codex|claude`. Skills supplement MCP registration and do not authenticate the CLI.

The integrated `dev-v08` binary passed `web/tests/agent_guides.py` on 2026-11-19: all six clients, light/dark themes, widths 1440/390/320, token-inclusive clipboard flow, token reuse on recopy/shell change, revoked-token renewal with 30-day share/review opt-ins, keyboard selection, and all five locales. All 36 axe color-contrast states passed with no browser page errors. Evidence is in `.work/ux-v08/agent-guides/`; captures contain authored fixture data and masked credentials. The separate disposable quick-connect smoke also forced clipboard denial twice and confirmed no additional token issuance.
