#!/usr/bin/env python3
"""Render deployment safety contracts; optionally smoke-test isolated Docker data.

Requires Docker Compose, kubectl and PyYAML. Never applies cluster resources.
"""

import argparse
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

import yaml


ROOT = Path(__file__).resolve().parents[1]


def run(*args, env=None, data=None):
    result = subprocess.run(args, cwd=ROOT, env=env, input=data, capture_output=True)
    if result.returncode:
        # Avoid dumping container logs, Secret contents or setup responses.
        raise AssertionError(f"Command failed ({result.returncode}): {args[0]} {args[1]}")
    return result.stdout


def check_configs():
    env = dict(os.environ, FOLIO_IMAGE="folio:deployment-test", FOLIO_APP_URL="https://folio.test:8443",
               FOLIO_CONTENT_URL="", FOLIO_PORT="4310", FOLIO_CONTENT_PORT="4311")
    compose = json.loads(run("docker", "compose", "config", "--format", "json", env=env))
    service = compose["services"]["folio"]
    assert service["image"] == "folio:deployment-test"
    assert service["user"] == "10001:10001" and service["read_only"]
    assert service["cap_drop"] == ["ALL"]
    assert all(p["host_ip"] == "127.0.0.1" for p in service["ports"])
    assert service["environment"]["FOLIO_CONTENT_URL"] == ""
    health = service["healthcheck"]["test"][1].replace("$$", "$")
    assert "FOLIO_APP_URL#*://" in health and 'Host: $host' in health and "/api/bootstrap" in health
    assert any(v["target"] == "/data" and v["type"] == "volume" for v in service["volumes"])
    assert "FOLIO_BOOTSTRAP_TOKEN" not in service["environment"]

    for flavor in ("base", "overlays/ingress"):
        raw = run("kubectl", "kustomize", f"deploy/kubernetes/{flavor}")
        objects = list(yaml.safe_load_all(raw))
        by_kind = {o["kind"]: o for o in objects}
        assert "Secret" not in by_kind  # No committed bootstrap credential.
        deployment = by_kind["Deployment"]
        assert deployment["spec"]["replicas"] == 1
        assert deployment["spec"]["strategy"] == {"type": "Recreate"}
        spec = deployment["spec"]["template"]["spec"]
        assert not spec["automountServiceAccountToken"]
        assert spec["securityContext"]["runAsUser"] == 10001
        assert spec["securityContext"]["fsGroup"] == 10001
        app = spec["containers"][0]
        assert app["args"][-1] == "/data/folio"
        assert app["securityContext"]["readOnlyRootFilesystem"]
        assert app["env"][0]["valueFrom"]["secretKeyRef"]["optional"]
        url = by_kind["ConfigMap"]["data"]["FOLIO_APP_URL"]
        for name in ("startupProbe", "readinessProbe", "livenessProbe"):
            probe = app[name]["httpGet"]
            assert probe["path"] == "/api/bootstrap" and probe["port"] == "http"
            assert probe["httpHeaders"] == [{"name": "Host", "value": urllib.parse.urlsplit(url).netloc}]
        assert by_kind["Service"]["spec"]["type"] == "ClusterIP"
        assert by_kind["PersistentVolumeClaim"]["spec"]["accessModes"] == ["ReadWriteOnce"]
        if flavor == "base":
            assert "Ingress" not in by_kind
        else:
            ingress = by_kind["Ingress"]["spec"]
            assert ingress["rules"][0]["host"] == urllib.parse.urlsplit(url).hostname
            assert ingress["tls"][0]["hosts"] == [ingress["rules"][0]["host"]]
            assert ingress["rules"][0]["http"]["paths"][0]["backend"]["service"]["port"]["name"] == "http"
    print("PASS: Compose and both Kustomize configurations render; deployment safety contracts hold")


def http(port, path="/api/bootstrap", host="folio.localhost:4310", body=None):
    headers = {"Host": host}
    if body is not None:
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(f"http://127.0.0.1:{port}{path}", headers=headers,
                                     data=json.dumps(body).encode() if body is not None else None)
    # Ignore workstation HTTP proxies for isolated loopback checks.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(request, timeout=3) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.read()


def check_docker(image):
    suffix = uuid.uuid4().hex[:12]
    projects = [f"folio-deploy-test-{suffix}", f"folio-restore-test-{suffix}"]
    env = dict(os.environ, FOLIO_IMAGE=image, FOLIO_APP_URL="http://folio.localhost:4310",
               FOLIO_CONTENT_URL="", FOLIO_PORT="0", FOLIO_CONTENT_PORT="0")

    def compose(project, *args, data=None):
        return run("docker", "compose", "-p", project, "-f", str(ROOT / "compose.yaml"),
                   *args, env=env, data=data)

    def port(project, number):
        return int(compose(project, "port", "folio", str(number)).decode().strip().rsplit(":", 1)[1])

    def ready(project, setup):
        for _ in range(60):
            try:
                status, body = http(port(project, 4310))
                if status == 200 and json.loads(body)["needsSetup"] == setup:
                    return
            except (OSError, ValueError, AssertionError):
                pass
            time.sleep(1)
        raise AssertionError("Isolated Folio did not become ready")

    try:
        project = projects[0]
        compose(project, "up", "-d", "--no-build", "--pull", "never")
        ready(project, True)
        app_port = port(project, 4310)
        assert http(app_port, host="wrong.test")[0] == 404
        assert compose(project, "exec", "-T", "folio", "id", "-u").strip() == b"10001"
        mode = compose(project, "exec", "-T", "folio", "stat", "-c", "%a", "/data").strip()
        assert mode == b"700"
        # The exact Compose healthcheck must work with its Host derivation.
        health = json.loads(compose(project, "config", "--format", "json"))["services"]["folio"]["healthcheck"]["test"][1].replace("$$", "$")
        compose(project, "exec", "-T", "folio", "sh", "-ec", health)
        token = compose(project, "exec", "-T", "folio", "cat", "/data/bootstrap.token").decode().strip()
        installation_key = compose(project, "exec", "-T", "folio", "cat", "/data/data.key")
        password = secrets.token_urlsafe(24)
        status, _ = http(app_port, "/api/setup", body={"token": token, "organizationName": "Deployment test",
                         "name": "Test operator", "email": "operator@deployment.invalid", "password": password})
        assert status == 200, f"Setup failed: HTTP {status}"
        compose(project, "restart", "folio")
        ready(project, False)
        # Copy only stopped, isolated installation data into another fresh volume.
        compose(project, "stop", "folio")
        archive = compose(project, "run", "--rm", "--no-deps", "--entrypoint", "tar", "folio", "-C", "/data", "-czf", "-", ".")
        restored = projects[1]
        compose(restored, "run", "--rm", "--no-deps", "-T", "--entrypoint", "tar", "folio", "-C", "/data", "-xzf", "-", data=archive)
        compose(restored, "up", "-d", "--no-build", "--pull", "never")
        ready(restored, False)
        assert compose(restored, "exec", "-T", "folio", "cat", "/data/data.key") == installation_key
        assert http(port(restored, 4310), "/api/auth/login", body={
            "email": "operator@deployment.invalid", "password": password})[0] == 200
        # Exercise the separate content listener, without exposing management on it.
        env["FOLIO_CONTENT_URL"] = "http://{artifact}.content.test:4311"
        compose(restored, "up", "-d", "--no-build", "--pull", "never")
        ready(restored, False)
        assert http(port(restored, 4311), host="folio.localhost:4310")[0] == 404
        assert http(port(restored, 4311), host="missing.content.test:4311")[0] == 404
        print("PASS: isolated non-root Docker setup, Host rejection, healthcheck, restart, backup/restore and subdomain listener")
    finally:
        for project in projects:
            compose(project, "down", "--volumes", "--remove-orphans")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--docker-image", help="Already-built local image; creates fresh temporary test volumes")
    args = parser.parse_args()
    check_configs()
    if args.docker_image:
        check_docker(args.docker_image)
