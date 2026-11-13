#!/usr/bin/env python3
"""Fresh temporary server and real CLI acceptance; no public account required."""
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.request


def port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", default="dist/folio")
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())
    with tempfile.TemporaryDirectory(prefix="folio-cli-smoke-") as tmp:
        tmp = Path(tmp)
        app_port, content_port = port(), port()
        origin = f"http://127.0.0.1:{app_port}"
        env = dict(os.environ, FOLIO_CONFIG=str(tmp / "client.json"), FOLIO_URL=origin, FOLIO_BOOTSTRAP_TOKEN="temporary-smoke-setup")
        env.pop("FOLIO_TOKEN", None)
        log = (tmp / "server.log").open("w")
        process = subprocess.Popen([binary, "serve", "--data-dir", str(tmp / "data"), "--listen", f"127.0.0.1:{app_port}", "--content-listen", f"127.0.0.1:{content_port}", "--app-url", origin, "--content-url", f"http://{{artifact}}.content.localhost:{content_port}"], env=env, stdout=log, stderr=log)
        jar = http.cookiejar.CookieJar()
        browser = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
        csrf = ""
        def api(path, payload=None):
            req = urllib.request.Request(origin + path, data=None if payload is None else json.dumps(payload).encode(), headers={"Content-Type": "application/json", "Origin": origin, "X-CSRF-Token": csrf})
            with browser.open(req, timeout=5) as response:
                return json.load(response)
        def cli(*command, stdin=None):
            p = subprocess.run([binary, *command], input=stdin, env=env, capture_output=True, text=True, timeout=15)
            assert p.returncode == 0, p.stdout + p.stderr
            return json.loads(p.stdout) if p.stdout.strip() else {}
        try:
            for _ in range(100):
                try:
                    api("/api/bootstrap")
                    break
                except Exception:
                    assert process.poll() is None, (tmp / "server.log").read_text()
                    time.sleep(.05)
            setup = api("/api/setup", {"token": "temporary-smoke-setup", "organizationName": "CLI smoke", "name": "Admin", "email": "admin@example.test", "password": "temporary-password-123"})
            csrf = setup["csrfToken"]
            token = api("/api/tokens", {"name": "cli-smoke", "scopes": ["read", "publish", "share"]})["token"]
            cli("login", "--token-stdin", stdin=token + "\n")
            assert cli("doctor")["authenticated"]
            site = tmp / "site"
            site.mkdir()
            (site / "index.html").write_text("<!doctype html><title>CLI smoke</title><h1>Folio</h1>")
            created = cli("publish", "--title", "CLI smoke", str(site))
            artifact = created["artifact"]
            artifact_id = artifact["id"]
            assert artifact["visibility"] == "private"
            assert len(cli("list", "--view", "mine")["artifacts"]) == 1
            cli("share", "--visibility", "organization", artifact_id)
            cli("publish", "--artifact", artifact_id, "--base-version", artifact["currentVersionId"], str(site))
            assert len(cli("inspect", artifact_id)["versions"]) == 2
            assert cli("archive", artifact_id)["artifact"]["archived"]
            assert not cli("restore", artifact_id)["artifact"]["archived"]
            skill_dir = tmp / "skills"
            cli("skills", "install", "--dir", str(skill_dir))
            assert "name: folio" in (skill_dir / "folio" / "SKILL.md").read_text()
        finally:
            process.terminate()
            process.wait(timeout=15)
            log.close()
        assert process.returncode == 0, "server did not shut down cleanly"
    print("PASS: fresh setup, private login config, doctor, CLI publish/revision/share/archive/restore, skill install, graceful shutdown")


if __name__ == "__main__":
    main()
