#!/usr/bin/env python3
"""Agent guide acceptance on a disposable installation; never runs client registration."""
import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
from playwright.sync_api import sync_playwright, expect


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
        'Codex': 'codex mcp add folio -- folio mcp',
        'Claude Code': 'claude mcp add --scope user --transport stdio folio -- folio mcp',
        'Gemini CLI': 'gemini mcp add --scope user --transport stdio folio folio mcp',
        'VS Code · Copilot': 'code --add-mcp',
        'Cursor': 'cursor "$HOME/.cursor/mcp.json"',
        'OpenCode': 'opencode mcp add folio -- folio mcp',
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
                            expect(guide.locator('.agent-client-instructions pre').first).to_contain_text(command)
                            expect(guide.locator('.agent-client-instructions')).to_contain_text('folio_capabilities')
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
                checks.append('six-agent-commands-both-themes-three-widths')
                # Clipboard reflects the active agent and shell, never an API token.
                page.set_viewport_size({'width': 1440, 'height': 1100})
                guide.get_by_role('button', name='Codex', exact=True).click()
                guide.get_by_role('button', name='등록 명령 복사', exact=True).click()
                assert page.evaluate('navigator.clipboard.readText()') == registrations['Codex']
                guide.get_by_role('button', name='Windows · PowerShell', exact=True).click()
                expect(guide.locator('.agent-setup-step').first).to_contain_text('Read-Host')
                guide.get_by_role('button', name='VS Code · Copilot', exact=True).click()
                expect(guide.locator('.agent-client-instructions pre').first).to_contain_text('code --% --add-mcp')
                guide.get_by_role('button', name='로그인 명령 복사', exact=True).click()
                copied = page.evaluate('navigator.clipboard.readText()')
                assert '--token-stdin' in copied and app in copied and 'Read-Host' in copied
                checks.append('copy-follows-selected-agent-and-shell')
                # Native buttons provide Tab/Space interaction without a custom tab model.
                button = guide.get_by_role('button', name='Gemini CLI', exact=True)
                button.focus()
                page.keyboard.press('Space')
                expect(button).to_have_attribute('aria-pressed', 'true')
                checks.append('keyboard-selection')
                catalogs = {locale: json.loads(Path(f'web/src/locales/{locale}.json').read_text()) for locale in ['ko', 'en', 'ja', 'zh-CN', 'fr']}
                for locale, catalog in catalogs.items():
                    page.evaluate('(locale) => localStorage.setItem("folio.locale", locale)', locale)
                    page.reload()
                    expect(guide.get_by_role('heading', name=catalog['2. 에이전트 연결'], exact=True)).to_be_visible()
                    expect(guide.get_by_role('button', name=catalog['로그인 명령 복사'], exact=True)).to_be_visible()
                    for name in registrations:
                        guide.get_by_role('button', name=name, exact=True).click()
                        expect(guide.locator('.agent-client-heading h4')).to_have_text(catalog['{agent}에 등록'].replace('{agent}', name))
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
