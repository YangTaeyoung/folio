#!/usr/bin/env python3
"""Exercise installer verification/idempotency against a local release fixture.

Uses the actual built executable, a temporary install directory, and a fake
download command. No public release/network or user's installed binary changes.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import tarfile
import tempfile


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", default="dist/folio")
    args = parser.parse_args()
    binary = Path(args.binary).resolve()
    assert binary.is_file(), "Build Folio first"
    root = Path(__file__).resolve().parents[1]
    with tempfile.TemporaryDirectory(prefix="folio-installer-test-") as tmp:
        tmp = Path(tmp)
        fixtures = tmp / "fixtures"
        fixtures.mkdir()
        arch = {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64"}[platform.machine()]
        name = f"folio_0.1.0_{platform.system().lower()}_{arch}.tar.gz"
        with tarfile.open(fixtures / name, "w:gz") as archive:
            archive.add(binary, arcname="folio")
        checksum = hashlib.sha256((fixtures / name).read_bytes()).hexdigest()
        manifest = fixtures / "checksums.txt"
        manifest.write_text(f"{checksum}  {name}\n")
        downloads = tmp / "download-bin"
        downloads.mkdir()
        curl = downloads / "curl"
        curl.write_text("#!/usr/bin/env python3\n" +
            "import os, pathlib, sys\n" +
            "args=sys.argv[1:]; url=next(a for a in args if a.startswith('https://'))\n" +
            "if '/releases/latest' in url: print('{\"tag_name\":\"v0.1.0\"}');sys.exit(0)\n" +
            "source=pathlib.Path(os.environ['FOLIO_TEST_FIXTURES'])/url.rsplit('/',1)[1]\n" +
            "pathlib.Path(args[args.index('-o')+1]).write_bytes(source.read_bytes())\n")
        curl.chmod(0o755)
        install_dir = tmp / "install"
        env = dict(os.environ, PATH=str(downloads) + os.pathsep + os.environ["PATH"],
                   FOLIO_INSTALL_DIR=str(install_dir), FOLIO_TEST_FIXTURES=str(fixtures))
        env.pop("FOLIO_VERSION", None)
        env.pop("FOLIO_INSTALL_FORCE", None)
        def install(success=True):
            p = subprocess.run(["sh", str(root / "scripts/install.sh")], env=env, capture_output=True, text=True)
            assert (p.returncode == 0) == success, p.stdout + p.stderr
            return p
        install()
        assert (install_dir / "folio").read_bytes() == binary.read_bytes()
        assert "already installed" in install().stdout
        (install_dir / "folio").write_bytes(b"existing file")
        install(False)
        assert (install_dir / "folio").read_bytes() == b"existing file"
        manifest.write_text(f"{'0' * 64}  {name}\n")
        env["FOLIO_INSTALL_FORCE"] = "1"
        p = install(False)
        assert "Checksum mismatch" in p.stderr
        assert (install_dir / "folio").read_bytes() == b"existing file"
        manifest.write_text(f"{checksum}  {name}\n")
        install()
        assert (install_dir / "folio").read_bytes() == binary.read_bytes()
    print("PASS: checksum verification, install, idempotency, existing-file preservation and explicit upgrade")


if __name__ == "__main__":
    main()
