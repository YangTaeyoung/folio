import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  mkdtempSync,
  writeFileSync,
  readFileSync,
  existsSync,
  rmSync,
} from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  loginCommand,
  connectionCommand,
  registrationCommand,
  quotePosix,
  quotePowerShell,
  cursorConfiguration,
} from "../src/agentClientCommands.ts";

test("login sends the token through stdin and keeps the origin literal in Bash and zsh", () => {
  const directory = mkdtempSync(join(tmpdir(), "folio-command-test-"));
  try {
    // A real shell launches only this disposable stub, never an installed Folio/client.
    writeFileSync(
      join(directory, "folio"),
      '#!/bin/bash\nprintf "%s\\0" "$@" > "$FOLIO_TEST_ARGS"\ncat > "$FOLIO_TEST_STDIN"\nexit "${FOLIO_TEST_EXIT:-0}"\n',
      { mode: 0o700 },
    );
    const marker = join(directory, "injected");
    const origin = `https://folio.example/'\"; touch ${marker}; $(touch ${marker})`;
    const token = "fixture-only-token-$`'\\";
    assert.ok(
      existsSync("/bin/bash"),
      "Bash is required for the displayed login command",
    );
    for (const shell of ["/bin/bash", "/bin/zsh"].filter(existsSync)) {
      for (const code of [0, 19]) {
        const result = spawnSync(
          shell,
          ["-f", "-c", loginCommand(origin, "posix")],
          {
            env: {
              ...process.env,
              PATH: `${directory}:/usr/bin:/bin`,
              ZDOTDIR: directory,
              BASH_ENV: "/dev/null",
              FOLIO_TEST_ARGS: join(directory, "args"),
              FOLIO_TEST_STDIN: join(directory, "stdin"),
              FOLIO_TEST_EXIT: String(code),
            },
            input: token + "\n",
            encoding: "utf8",
          },
        );
        assert.equal(result.status, code, result.stderr);
        assert.deepEqual(
          readFileSync(join(directory, "args"), "utf8").split("\0"),
          ["login", "--url", origin, "--token-stdin", ""],
        );
        assert.equal(readFileSync(join(directory, "stdin"), "utf8"), token);
        assert.equal(result.stdout.includes(token), false);
        assert.equal(existsSync(marker), false);
      }
    }
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("registration commands pass only the local stdio launcher, without credentials", () => {
  for (const shell of ["posix", "powershell"] as const) {
    for (const client of [
      "codex",
      "claudecode",
      "antigravity",
      "vscode",
      "cursor",
      "opencode",
    ] as const) {
      const command = registrationCommand(client, shell);
      assert.ok(command.length > 0);
      assert.doesNotMatch(
        command,
        /FOLIO_TOKEN|API_TOKEN|YOUR_API_TOKEN|--trust|--approve/,
      );
    }
  }
  assert.equal(
    registrationCommand("opencode", "posix"),
    "opencode mcp add folio -- folio mcp",
  );
  assert.equal(
    registrationCommand("cursor", "posix"),
    "folio connect --client cursor",
  );
  assert.equal(
    registrationCommand("antigravity", "posix"),
    "agy mcp add --type stdio folio folio mcp",
  );
  const args = spawnSync(
    "/bin/bash",
    [
      "-c",
      `set -- ${registrationCommand("vscode", "posix")}; printf '%s' "$3"`,
    ],
    { encoding: "utf8" },
  );
  assert.deepEqual(JSON.parse(args.stdout), {
    name: "folio",
    type: "stdio",
    command: "folio",
    args: ["mcp"],
  });
  assert.deepEqual(JSON.parse(cursorConfiguration), {
    mcpServers: { folio: { command: "folio", args: ["mcp"] } },
  });
});

test("PowerShell login quotes data and clears its temporary secret allocation", () => {
  assert.equal(
    quotePowerShell("it's $literal; `data`"),
    "'it''s $literal; `data`'",
  );
  assert.equal(quotePosix("a'b"), `'a'"'"'b'`);
  const command = loginCommand("https://folio.example/a'b", "powershell");
  assert.match(command, /Read-Host 'Folio API token' -AsSecureString/);
  assert.match(command, /--url 'https:\/\/folio.example\/a''b' --token-stdin/);
  assert.match(command, /finally \{/);
  assert.match(command, /ZeroFreeBSTR/);
  assert.match(command, /Remove-Variable folioSecret, folioPointer/);
  assert.match(
    registrationCommand("vscode", "powershell"),
    /^code --% --add-mcp \{\\"name\\":/,
  );
});

test("one pasted connection script preserves token data and stops on login or registration failure", () => {
  const directory = mkdtempSync(join(tmpdir(), "folio-connect-command-"));
  try {
    const stub =
      '#!/bin/bash\nprintf "%s\\0" "$0" "$@" >> "$FOLIO_TEST_ARGS"\nif [ "$1" = login ]; then cat > "$FOLIO_TEST_STDIN"; exit "${FOLIO_TEST_LOGIN_EXIT:-0}"; fi\nif [ "$1" = mcp ] && [ "$2" = add ]; then exit "${FOLIO_TEST_REGISTER_EXIT:-0}"; fi\n';
    for (const executable of [
      "folio",
      "codex",
      "claude",
      "agy",
      "code",
      "opencode",
    ]) {
      writeFileSync(join(directory, executable), stub, { mode: 0o700 });
    }
    const marker = join(directory, "injected");
    const origin = `https://folio.example/'\"; touch ${marker}; $(touch ${marker})`;
    const token = `folio_fixture-'$\`\\$(touch ${marker})`;
    const guard = ["folio", "codex", "claude", "agy", "code", "opencode"]
      .map(
        (name) =>
          `test "$(command -v ${name})" = ${quotePosix(join(directory, name))} || exit 88`,
      )
      .join("\n");
    for (const shell of ["/bin/bash", "/bin/zsh"].filter(existsSync)) {
      for (const client of [
        "codex",
        "claudecode",
        "antigravity",
        "vscode",
        "cursor",
        "opencode",
      ] as const) {
        for (const loginExit of [0, 19]) {
          writeFileSync(join(directory, "args"), "");
          const command = connectionCommand(client, origin, token, "posix");
          const result = spawnSync(
            shell,
            ["-f", "-c", `${guard}\n${command}`],
            {
              env: {
                ...process.env,
                PATH: `${directory}:/usr/bin:/bin`,
                ZDOTDIR: directory,
                BASH_ENV: "/dev/null",
                FOLIO_TEST_ARGS: join(directory, "args"),
                FOLIO_TEST_STDIN: join(directory, "stdin"),
                FOLIO_TEST_LOGIN_EXIT: String(loginExit),
              },
              encoding: "utf8",
            },
          );
          assert.equal(result.status, loginExit, result.stderr);
          const calls = readFileSync(join(directory, "args"), "utf8");
          assert.equal(readFileSync(join(directory, "stdin"), "utf8"), token);
          assert.ok(calls.includes(origin));
          assert.equal(
            calls.includes(token),
            false,
            "token must not enter native executable argv",
          );
          assert.equal(calls.includes("doctor"), loginExit === 0);
          assert.equal(
            calls
              .split("\0")
              .some(
                (argument) => argument === "add" || argument === "--add-mcp",
              ),
            loginExit === 0 && client !== "cursor",
            `${shell} ${client} exit=${loginExit}: ${JSON.stringify(calls.split("\0"))}`,
          );
          assert.equal(existsSync(marker), false);
          assert.equal(result.stdout.includes(token), false);
        }
      }
    }
    writeFileSync(join(directory, "args"), "");
    const failed = spawnSync(
      "/bin/bash",
      [
        "-f",
        "-c",
        `${guard}\n${connectionCommand("codex", "https://folio.example", token, "posix")}`,
      ],
      {
        env: {
          ...process.env,
          PATH: `${directory}:/usr/bin:/bin`,
          ZDOTDIR: directory,
          BASH_ENV: "/dev/null",
          FOLIO_TEST_ARGS: join(directory, "args"),
          FOLIO_TEST_STDIN: join(directory, "stdin"),
          FOLIO_TEST_REGISTER_EXIT: "23",
        },
        encoding: "utf8",
      },
    );
    assert.equal(failed.status, 23);
    assert.equal(
      readFileSync(join(directory, "args"), "utf8").includes("doctor"),
      false,
    );
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("PowerShell one-paste script isolates variables and checks every native exit status", () => {
  const command = connectionCommand(
    "antigravity",
    "https://folio.example/a'b",
    "folio_'$literal",
    "powershell",
  );
  assert.match(command, /^& \{/);
  assert.match(command, /\$folioToken = 'folio_''\$literal'/);
  assert.match(
    command,
    /\$folioToken \| folio login --url 'https:\/\/folio.example\/a''b' --token-stdin/,
  );
  assert.match(command, /agy mcp add --type stdio folio folio mcp/);
  assert.match(command, /Remove-Variable folioToken/);
  assert.equal((command.match(/LASTEXITCODE/g) || []).length, 4);
});
