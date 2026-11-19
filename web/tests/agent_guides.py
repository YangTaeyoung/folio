#!/usr/bin/env python3
"""Agent guide acceptance on a disposable installation; never runs client registration."""
import argparse
import json
import os
import re
from pathlib import Path
import socket
import subprocess
import tempfile
import time
from playwright.sync_api import sync_playwright, expect
from browser_controls import choose


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--screenshots', default='.work/theme/agent-guides')
    args = parser.parse_args()
    output = Path(args.screenshots).resolve()
    output.mkdir(parents=True, exist_ok=True)
    app_port, content_port = free_port(), free_port()
    app = f'http://folio.localhost:{app_port}'
    errors, checks, audits = [], [], []
    registrations = {
        'Codex': 'folio connect --client codex',
        'Claude Code': 'folio connect --client claudecode',
        'Antigravity CLI': 'folio connect --client antigravity',
        'VS Code · Copilot': 'folio connect --client vscode',
        'Cursor': 'folio connect --client cursor',
        'OpenCode': 'folio connect --client opencode',
    }
    with tempfile.TemporaryDirectory(prefix='folio-agent-guides-') as data, open(output/'server.log', 'w+') as log:
        process = subprocess.Popen([
            str(Path(args.binary).resolve()), 'serve', '--data-dir', data,
            '--listen', f'127.0.0.1:{app_port}', '--content-listen', f'127.0.0.1:{content_port}',
            '--app-url', app, '--content-url', f'http://{{artifact}}.content.folio.localhost:{content_port}',
        ], env={**os.environ, 'FOLIO_BOOTSTRAP_TOKEN': 'agent-guide-authored-fixture'}, stdout=log, stderr=log)
        try:
            with sync_playwright() as pw:
                browser = pw.chromium.launch()
                context = browser.new_context(viewport={'width': 1440, 'height': 1100}, permissions=['clipboard-read', 'clipboard-write'])
                for _ in range(100):
                    try:
                        if context.request.get(app+'/api/bootstrap', timeout=500).ok:
                            break
                    except Exception:
                        time.sleep(.1)
                response = context.request.post(app+'/api/setup', headers={'Origin': app}, data={
                    'token': 'agent-guide-authored-fixture', 'organizationName': 'Agent guide fixture',
                    'name': 'Guide Admin', 'email': 'guide@folio.test', 'password': 'Guide-fixture-only-123!',
                })
                assert response.ok, response.text()
                assert context.request.get(app+'/api/tokens').json()['tokens'] == []
                page = context.new_page()
                page.on('pageerror', lambda error: errors.append(str(error)))
                page.goto(app+'/#agents')
                guide = page.locator('.agent-client-guides')
                expect(guide).to_be_visible()
                for mode in ['light', 'dark']:
                    page.evaluate('(mode) => { localStorage.setItem("folio.theme", mode); }', mode)
                    page.reload()
                    expect(page.locator('html')).to_have_attribute('data-theme', mode)
                    page.evaluate(Path('web/node_modules/axe-core/axe.min.js').read_text())
                    for width in [1440, 390, 320]:
                        page.set_viewport_size({'width': width, 'height': 1100})
                        for name, command in registrations.items():
                            button = guide.locator('.agent-client-options').get_by_role('button', name=name, exact=True)
                            button.click()
                            expect(button).to_have_attribute('aria-pressed', 'true')
                            expect(guide.locator('.agent-client-options button[aria-pressed="true"]')).to_have_count(1)
                            guide.locator('.agent-connection-action button').click()
                            expect(guide.locator('.agent-connection-action button')).to_have_text('다시 복사')
                            copied = page.evaluate('navigator.clipboard.readText()')
                            assert command in copied and '--token-stdin' in copied and app in copied and 'folio doctor' in copied
                            assert '"token": "folio_' in copied, 'connection command must include the newly issued fixture token'
                            expect(guide).to_contain_text('folio_capabilities')
                            # Credentials are not displayed unless explicitly revealed.
                            if guide.locator('.agent-command-disclosure').get_attribute('open') is None:
                                guide.locator('.agent-command-disclosure > summary').click()
                            fixture_token = re.search(r'"token": "(folio_[A-Za-z0-9_-]{43})"', copied).group(1)
                            assert fixture_token not in guide.locator('.agent-command-text').input_value()
                            assert '••••••••' in guide.locator('.agent-command-text').input_value()
                            assert 'YOUR_API_TOKEN' not in guide.inner_text()
                            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1'), (mode, width, name)
                            violations = page.evaluate("""async () => {
                                const result = await axe.run('.agent-client-guides', {runOnly:{type:'rule',values:['color-contrast']}});
                                return result.violations.map(v => ({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))}));
                            }""")
                            audits.append({'theme': mode, 'width': width, 'agent': name, 'violations': violations})
                            (output/'contrast.json').write_text(json.dumps(audits, indent=2, ensure_ascii=False))
                            assert not violations, (mode, width, name, violations)
                        page.screenshot(path=str(output/f'{mode}-{width}-agents.png'), full_page=True)
                checks.append('six-agent-prompts-both-themes-three-widths')
                # A second copy and a shell change reuse the same issued token.
                page.set_viewport_size({'width': 1440, 'height': 1100})
                guide.get_by_role('button', name='Codex', exact=True).click()
                before = context.request.get(app+'/api/tokens').json()['tokens']
                guide.locator('.agent-connection-action button').click()
                expect(guide.locator('.agent-connection-action button')).to_have_text('다시 복사')
                copied = page.evaluate('navigator.clipboard.readText()')
                assert registrations['Codex'] in copied and '--token-stdin' in copied and app in copied
                assert len(context.request.get(app+'/api/tokens').json()['tokens']) == len(before)
                guide.get_by_role('button', name='터미널에서 실행', exact=True).click()
                guide.get_by_role('button', name='Windows · PowerShell', exact=True).click()
                guide.get_by_role('button', name='VS Code · Copilot', exact=True).click()
                guide.locator('.agent-connection-action button').click()
                expect(guide.locator('.agent-connection-action button')).to_have_text('다시 복사')
                copied = page.evaluate('navigator.clipboard.readText()')
                assert copied.startswith('& {') and 'connect --client vscode' in copied and '--token-stdin' in copied
                assert len(context.request.get(app+'/api/tokens').json()['tokens']) == len(before)
                checks.append('single-copy-includes-token-login-registration-verification-and-reuses-token')
                guide.get_by_role('button', name='에이전트에 붙여넣기', exact=True).click()
                # Revocation unlocks options; a new explicit copy applies opt-in scopes.
                csrf = response.json()['csrfToken']
                records = context.request.get(app+'/api/tokens').json()['tokens']
                cursor = [record for record in records if record['name'] == 'Folio · Cursor'][-1]
                revoked = context.request.delete(app+'/api/tokens/'+cursor['id'], headers={'Origin': app, 'X-CSRF-Token': csrf})
                assert revoked.status == 204
                guide.get_by_role('button', name='Cursor', exact=True).click()
                guide.locator('.agent-connection-options > summary').click()
                expect(guide.get_by_label('공유 권한 추가', exact=True)).to_be_enabled()
                choose(guide.get_by_label('토큰 만료', exact=True), '30')
                guide.get_by_label('공유 권한 추가', exact=True).check()
                guide.get_by_label('검토 권한 추가', exact=True).check()
                guide.locator('.agent-connection-action button').click()
                expect(guide.locator('.agent-connection-action button')).to_have_text('다시 복사')
                records = context.request.get(app+'/api/tokens').json()['tokens']
                fresh = [record for record in records if record['name'] == 'Folio · Cursor'][-1]
                assert fresh['id'] != cursor['id']
                assert fresh['scopes'] == ['read', 'publish', 'share', 'review']
                from datetime import datetime
                duration = datetime.fromisoformat(fresh['expiresAt'].replace('Z', '+00:00')) - datetime.fromisoformat(fresh['createdAt'].replace('Z', '+00:00'))
                assert abs(duration.total_seconds() - 30 * 86400) < 2
                checks.append('revoked-token-renewal-and-opt-in-scopes-expiry')
                # Native buttons provide Tab/Space interaction without a custom tab model.
                button = guide.get_by_role('button', name='Antigravity CLI', exact=True)
                button.focus()
                page.keyboard.press('Space')
                expect(button).to_have_attribute('aria-pressed', 'true')
                checks.append('keyboard-selection')
                catalogs = {locale: json.loads(Path(f'web/src/locales/{locale}.json').read_text()) for locale in ['ko', 'en', 'ja', 'zh-CN', 'fr']}
                for locale, catalog in catalogs.items():
                    page.evaluate('(locale) => localStorage.setItem("folio.locale", locale)', locale)
                    page.reload()
                    expect(guide.get_by_role('heading', name=catalog['사용할 에이전트 선택'], exact=True)).to_be_visible()
                    expect(guide.get_by_role('button', name=catalog['설정 프롬프트 복사'], exact=True)).to_be_visible()
                    for name in registrations:
                        guide.get_by_role('button', name=name, exact=True).click()
                        expect(guide.locator('.agent-client-options button[aria-pressed="true"]')).to_have_text(name)
                checks.append('all-five-locales-and-six-agents')
                assert not errors, errors
                result = {'status': 'PASS', 'checks': checks, 'pageErrors': errors, 'contrastStates': len(audits), 'axeContrastViolations': 0}
                (output/'result.json').write_text(json.dumps(result, indent=2))
                print(json.dumps(result))
                browser.close()
        finally:
            process.terminate()
            process.wait(timeout=15)


if __name__ == '__main__':
    main()
