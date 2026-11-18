#!/usr/bin/env python3
"""Native reader and navigation regressions on isolated authored installations.

Run: python web/tests/reader_navigation.py --binary dist/folio \
    --screenshots .work/reader-navigation
Tests both the default path content mode and explicit subdomain mode. Never runs
agent client commands or accesses existing user/application configuration.
"""
import argparse
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import tempfile
import time
from urllib.parse import parse_qs, urlparse

from playwright.sync_api import expect, sync_playwright


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def check_mode(browser, binary, output, mode):
    app_port, content_port = free_port(), free_port()
    app = f'http://folio.localhost:{app_port}'
    checks, page_errors, console_errors = [], [], []
    with tempfile.TemporaryDirectory(prefix=f'folio-reader-{mode}-') as data:
        with open(output / f'{mode}-server.log', 'w+') as log:
            command = [binary, 'serve', '--data-dir', data,
                       '--listen', f'127.0.0.1:{app_port}',
                       '--content-listen', f'127.0.0.1:{content_port}', '--app-url', app]
            if mode == 'subdomain':
                command += ['--content-url', f'http://{{artifact}}.content.folio.localhost:{content_port}']
            process = subprocess.Popen(command, env={**os.environ, 'FOLIO_BOOTSTRAP_TOKEN': 'reader-authored-fixture'},
                                       stdout=log, stderr=log)
            context = browser.new_context(viewport={'width': 1360, 'height': 900})
            try:
                for _ in range(100):
                    if process.poll() is not None:
                        log.seek(0)
                        raise RuntimeError(log.read())
                    try:
                        if context.request.get(app + '/api/bootstrap', timeout=500).ok:
                            break
                    except Exception:
                        time.sleep(.1)
                setup = context.request.post(app + '/api/setup', headers={'Origin': app}, data={
                    'token': 'reader-authored-fixture', 'organizationName': 'Reader fixture',
                    'name': 'Reader Admin', 'email': 'reader-admin@folio.test', 'password': 'Reader-fixture-only-123!'})
                assert setup.ok, setup.text()
                headers = {'Origin': app, 'X-CSRF-Token': setup.json()['csrfToken']}

                def upload(title, heading, artifact_id='', base_version='', draft=False):
                    path = f'/api/artifacts/{artifact_id}/versions' if artifact_id else '/api/artifacts'
                    response = context.request.post(app + path, headers=headers, multipart={
                        'metadata': json.dumps({'title': title, 'baseVersionId': base_version, 'draft': draft,
                                                'files': [{'path': 'index.html'}]}),
                        'file0': {'name': 'index.html', 'mimeType': 'text/html',
                                  'buffer': f'<!doctype html><title>{title}</title><h1>{heading}</h1>'.encode()}})
                    assert response.status == 201, (response.status, response.text())
                    return response.json()

                first = upload('Alpha report', 'Published revision')
                artifact_id, published_version = first['artifact']['id'], first['version']['id']
                upload('Beta report', 'Reference revision')
                page = context.new_page()
                page.on('pageerror', lambda error: page_errors.append(str(error)))

                def console_failure(message):
                    if message.type == 'error':
                        console_errors.append(re.sub(r'/grant/[^/?#\s]+', '/grant/[redacted]', message.text))

                page.on('console', console_failure)
                page.goto(app + '/#collections')
                page.get_by_role('button', name='새 컬렉션', exact=True).first.click()
                editor = page.locator('.collection-editor')
                editor.get_by_label('이름', exact=True).fill('Preserve this draft')
                editor.get_by_role('button', name='Alpha report 컬렉션에 추가', exact=True).click()
                create = page.locator('.workspace-feature-heading').get_by_role('button', name='새 컬렉션', exact=True)
                expect(create).to_be_disabled()
                create.evaluate('(button) => button.click()')
                expect(editor.get_by_label('이름', exact=True)).to_have_value('Preserve this draft')
                expect(editor.locator('.collection-order li')).to_have_count(1)
                checks.append('new-collection-does-not-reset-unsaved-name-or-selection')
                editor.get_by_role('button', name='취소', exact=True).click()

                page.goto(app + '/#library')
                expect(page.locator('.artifact-card')).to_have_count(2)
                page.goto(app + '/#agents')
                expect(page.locator('.agent-client-guides')).to_be_visible()
                upload('Published while away', 'External publication')
                page.go_back()
                expect(page.locator('.artifact-card')).to_have_count(3)
                checks.append('browser-back-refreshes-library-after-external-publication')

                draft = upload('Alpha draft', 'Draft revision', artifact_id, published_version, True)
                draft_version = draft['version']['id']
                page.goto(app)
                card = page.get_by_role('link', name='Alpha draft 열기', exact=True)
                expect(card).to_be_visible()
                href = card.get_attribute('href')
                assert parse_qs(urlparse(href).query).get('version') == [draft_version]
                card.click()
                expect(page).to_have_url(href)
                expect(page.locator('.viewer-header > span')).to_have_text('Alpha draft')
                expect(page.frame_locator('.viewer-frame').get_by_role('heading', name='Draft revision', exact=True)).to_be_visible()
                page.screenshot(path=str(output / f'{mode}-owner-draft-reader.png'), full_page=True, animations='disabled')
                checks.append('native-library-link-opens-explicit-owner-draft')

                stable_url = first['artifact']['shareUrl']
                stable_metadata = context.request.get(app + '/api/share/' + first['artifact']['slug'])
                assert stable_metadata.ok, stable_metadata.text()
                published = stable_metadata.json()['artifact']
                assert published['title'] == 'Alpha report'
                assert published['currentVersionId'] == published_version
                page.goto(stable_url)
                assert not urlparse(page.url).query
                expect(page.locator('.viewer-header > span')).to_have_text('Alpha report')
                expect(page.frame_locator('.viewer-frame').get_by_role('heading', name='Published revision', exact=True)).to_be_visible()
                page.screenshot(path=str(output / f'{mode}-stable-published-reader.png'), full_page=True, animations='disabled')
                checks.append('stable-share-metadata-and-render-both-use-published-version')

                collection = context.request.post(app + '/api/collections', headers=headers, data={
                    'name': 'Draft reader check', 'description': '', 'visibility': 'private', 'artifactIds': [artifact_id]})
                assert collection.status == 201, collection.text()
                page.goto(app + '/#collections')
                link = page.locator('.collection-artifact-open').first
                expect(link).to_contain_text('Alpha draft')
                href = link.get_attribute('href')
                assert parse_qs(urlparse(href).query).get('version') == [draft_version]
                link.click()
                expect(page).to_have_url(href)
                expect(page.locator('.viewer-header > span')).to_have_text('Alpha draft')
                expect(page.frame_locator('.viewer-frame').get_by_role('heading', name='Draft revision', exact=True)).to_be_visible()
                checks.append('native-collection-link-opens-explicit-owner-draft')
                assert not page_errors, page_errors
                assert not console_errors, console_errors
                return {'mode': mode, 'checks': checks, 'pageErrors': page_errors, 'consoleErrors': console_errors}
            finally:
                context.close()
                process.terminate()
                process.wait(timeout=15)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--screenshots', default='.work/reader-navigation')
    parser.add_argument('--content-mode', choices=['path', 'subdomain', 'both'], default='both')
    args = parser.parse_args()
    output = Path(args.screenshots).resolve()
    output.mkdir(parents=True, exist_ok=True)
    modes = ['path', 'subdomain'] if args.content_mode == 'both' else [args.content_mode]
    with sync_playwright() as pw:
        browser = pw.chromium.launch()
        try:
            results = [check_mode(browser, str(Path(args.binary).resolve()), output, mode) for mode in modes]
            result = {'status': 'PASS', 'modes': results}
            (output / 'result.json').write_text(json.dumps(result, ensure_ascii=False, indent=2))
            print(json.dumps(result, ensure_ascii=False))
        finally:
            browser.close()


if __name__ == '__main__':
    main()
