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
  for (const client of [
    "codex",
    "claudecode",
    "antigravity",
    "vscode",
    "cursor",
    "opencode",
  ] as const) {
    assert.equal(
      registrationCommand(client, "posix"),
      `folio connect --client ${client}`,
    );
    assert.equal(
      registrationCommand(client, "powershell"),
      `folio connect --client ${client}`,
    );
  }
  assert.equal(JSON.parse(cursorConfiguration).mcpServers.folio.args[0], "mcp");
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
    /^folio connect --client vscode$/,
  );
});

test("one pasted connection script preserves token data and stops on login or registration failure", () => {
  const directory = mkdtempSync(join(tmpdir(), "folio-connect-command-"));
  try {
    const stub =
      '#!/bin/bash\nprintf "%s\\0" "$0" "$@" >> "$FOLIO_TEST_ARGS"\nif [ "$1" = login ]; then cat > "$FOLIO_TEST_STDIN"; exit "${FOLIO_TEST_LOGIN_EXIT:-0}"; fi\nif [ "$1" = connect ]; then exit "${FOLIO_TEST_REGISTER_EXIT:-0}"; fi\n';
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
            calls.split("\0").some((argument) => argument === "connect"),
            loginExit === 0,
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
    /\$folioToken \| & \$folioExecutable login --url 'https:\/\/folio.example\/a''b' --token-stdin/,
  );
  assert.match(command, /& \$folioExecutable connect --client antigravity/);
  assert.match(command, /Remove-Variable folioToken/);
  assert.equal((command.match(/LASTEXITCODE/g) || []).length, 4);
});

test("missing client prints a useful error before touching credentials", () => {
  const directory = mkdtempSync(join(tmpdir(), "folio-missing-client-"));
  try {
    const calls = join(directory, "calls");
    writeFileSync(
      join(directory, "folio"),
      '#!/bin/bash\nprintf called > "$FOLIO_TEST_ARGS"\n',
      { mode: 0o700 },
    );
    const token = "folio_missing-client-fixture";
    const result = spawnSync(
      "/bin/bash",
      [
        "-f",
        "-c",
        connectionCommand("codex", "https://folio.example", token, "posix"),
      ],
      {
        env: {
          ...process.env,
          PATH: `${directory}:/usr/bin:/bin`,
          BASH_ENV: "/dev/null",
          FOLIO_TEST_ARGS: calls,
        },
        encoding: "utf8",
      },
    );
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /codex.*(install|found)/i);
    assert.equal(existsSync(calls), false);
    assert.equal((result.stdout + result.stderr).includes(token), false);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("a Folio binary outside PATH is found without running an installer", () => {
  const directory = mkdtempSync(join(tmpdir(), "folio-local-bin-"));
  try {
    const bin = join(directory, "clients"),
      local = join(directory, "local");
    spawnSync("/bin/mkdir", [bin, local]);
    writeFileSync(join(bin, "codex"), "#!/bin/sh\nexit 0\n", { mode: 0o700 });
    writeFileSync(
      join(local, "folio"),
      '#!/bin/sh\ncase "$1" in version) echo "Folio v0.8.2";; login) cat > "$FOLIO_TEST_STDIN";; connect) printf "%s\\n" "$@" > "$FOLIO_TEST_ARGS";; esac\n',
      { mode: 0o700 },
    );
    const result = spawnSync(
      "/bin/bash",
      [
        "-f",
        "-c",
        connectionCommand(
          "codex",
          "https://folio.example",
          "fixture-only-secret",
          "posix",
        ),
      ],
      {
        env: {
          ...process.env,
          PATH: `${bin}:/usr/bin:/bin`,
          FOLIO_INSTALL_DIR: local,
          BASH_ENV: "/dev/null",
          FOLIO_TEST_STDIN: join(directory, "stdin"),
          FOLIO_TEST_ARGS: join(directory, "args"),
        },
        encoding: "utf8",
      },
    );
    assert.equal(result.status, 0, result.stderr);
    assert.equal(
      readFileSync(join(directory, "stdin"), "utf8"),
      "fixture-only-secret",
    );
    assert.equal(
      readFileSync(join(directory, "args"), "utf8"),
      "connect\n--client\ncodex\n",
    );
    assert.match(result.stdout, /Folio connected/);
    assert.doesNotMatch(result.stdout, /fixture-only-secret/);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("a failed CLI download stops visibly before login or MCP registration", () => {
  const directory = mkdtempSync(join(tmpdir(), "folio-download-failure-"));
  try {
    writeFileSync(
      join(directory, "codex"),
      '#!/bin/sh\nprintf called > "$FOLIO_TEST_ARGS"\n',
      { mode: 0o700 },
    );
    writeFileSync(join(directory, "curl"), "#!/bin/sh\nexit 22\n", {
      mode: 0o700,
    });
    const result = spawnSync(
      "/bin/bash",
      [
        "-f",
        "-c",
        connectionCommand(
          "codex",
          "https://folio.example",
          "fixture-private-token",
          "posix",
        ),
      ],
      {
        env: {
          ...process.env,
          PATH: `${directory}:/usr/bin:/bin`,
          BASH_ENV: "/dev/null",
          FOLIO_INSTALL_DIR: join(directory, "empty"),
          FOLIO_TEST_ARGS: join(directory, "called"),
        },
        encoding: "utf8",
      },
    );
    assert.equal(result.status, 22, result.stderr);
    assert.match(result.stderr, /installing Folio CLI/);
    assert.equal(existsSync(join(directory, "called")), false);
    assert.equal(
      (result.stdout + result.stderr).includes("fixture-private-token"),
      false,
    );
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
