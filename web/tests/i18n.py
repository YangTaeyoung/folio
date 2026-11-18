#!/usr/bin/env python3
"""Five-language UI acceptance on a disposable Folio installation.
Run: python web/tests/i18n.py --binary .work/collaboration/folio --screenshots .work/collaboration/i18n
All accounts, artifacts and passwords are authored fixtures in a temporary directory.
"""
import argparse
import datetime
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import tempfile
import time
from browser_controls import choose as select_value
from playwright.sync_api import sync_playwright, expect


LOCALES = ['ko', 'en', 'ja', 'zh-CN', 'fr']
USER_TEXT = ['사용자 콘텐츠 제목', '사용자 콘텐츠 설명', '사용자 조직', '사용자 관리자', '사용자 컬렉션', '사용자 댓글', '사용자 태그']


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--screenshots', default='.work/collaboration/i18n')
    args = parser.parse_args()
    output = Path(args.screenshots).resolve()
    output.mkdir(parents=True, exist_ok=True)
    catalogs = {locale: json.loads((Path(__file__).parent.parent / 'src' / 'locales' / f'{locale}.json').read_text()) for locale in LOCALES}
    checks, errors, requests = [], [], []
    app_port, content_port = free_port(), free_port()
    app = f'http://folio.localhost:{app_port}'
    with tempfile.TemporaryDirectory(prefix='folio-i18n-') as data:
        with open(Path(data) / 'server.log', 'w+') as log:
            proc = subprocess.Popen([str(Path(args.binary).resolve()), 'serve', '--data-dir', data,
                '--listen', f'127.0.0.1:{app_port}', '--content-listen', f'127.0.0.1:{content_port}',
                '--app-url', app, '--content-url', f'http://{{artifact}}.content.folio.localhost:{content_port}'],
                env={**os.environ, 'FOLIO_BOOTSTRAP_TOKEN': 'i18n-fixture-only-setup'}, stdout=log, stderr=log)
            try:
                with sync_playwright() as playwright:
                    browser = playwright.chromium.launch(headless=True)
                    fixture = browser.new_context()
                    for _ in range(100):
                        if proc.poll() is not None:
                            log.seek(0)
                            raise RuntimeError(log.read())
                        try:
                            if fixture.request.get(app + '/api/bootstrap', timeout=500).ok:
                                break
                        except Exception:
                            time.sleep(.1)
                    password = 'I18n-fixture-only-password-123!'
                    setup = fixture.request.post(app + '/api/setup', headers={'Origin': app}, data={
                        'token': 'i18n-fixture-only-setup', 'organizationName': USER_TEXT[2], 'name': USER_TEXT[3],
                        'email': 'i18n@folio.test', 'password': password})
                    assert setup.ok, setup.text()
                    headers = {'Origin': app, 'X-CSRF-Token': setup.json()['csrfToken']}
                    def api(method, path, body=None, want=200):
                        response = fixture.request.fetch(app + path, method=method, headers=headers, data=body)
                        assert response.status == want, (path, response.status, response.text())
                        return response.json() if response.status != 204 else None
                    def upload(number, aid='', base=''):
                        html = f'<!doctype html><html lang="ko"><meta charset="utf-8"><h1>{USER_TEXT[0]} {number}</h1><p>{USER_TEXT[1]}</p></html>'
                        response = fixture.request.post(app + '/api/artifacts' + (f'/{aid}/versions' if aid else ''), headers=headers, multipart={
                            'metadata': json.dumps({'title': USER_TEXT[0], 'description': USER_TEXT[1], 'baseVersionId': base, 'files': [{'path': 'index.html'}, {'path': 'notes.txt'}]}),
                            'file0': {'name': 'index.html', 'mimeType': 'text/html', 'buffer': html.encode()},
                            'file1': {'name': 'notes.txt', 'mimeType': 'text/plain', 'buffer': (f'Line {number}\n' + 'x' * 1536).encode()}})
                        assert response.status == 201, response.text()
                        return response.json()
                    first = upload(1)
                    aid, v1 = first['artifact']['id'], first['version']['id']
                    second = upload(2, aid, v1)
                    v2 = second['version']['id']
                    policy = api('GET', '/api/admin/settings')['policy']
                    policy['quotaBytes'] = 1610612736
                    api('PATCH', '/api/admin/settings', {'policy': policy})
                    api('PATCH', f'/api/artifacts/{aid}/metadata', {'tags': [USER_TEXT[6]], 'provenance': {'agent': 'Fixture agent', 'model': 'fixture-model', 'summary': 'Authored test request', 'sources': ['https://example.com/reference']}})
                    api('POST', f'/api/artifacts/{aid}/comments', {'versionId': v2, 'body': USER_TEXT[5]}, want=201)
                    api('POST', '/api/collections', {'name': USER_TEXT[4], 'description': USER_TEXT[1], 'visibility': 'organization', 'artifactIds': [aid]}, want=201)
                    expiry = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=1)).isoformat().replace('+00:00', 'Z')
                    guest = api('POST', f'/api/artifacts/{aid}/guest-links', {'versionId': v2, 'expiresAt': expiry, 'password': 'Guest-fixture-only-123!'}, want=201)
                    guest_url = guest.get('guestLink', guest).get('url')
                    assert guest_url, guest

                    def install_watchers(page):
                        page.on('pageerror', lambda error: errors.append(str(error)))
                        page.on('request', lambda request: requests.append(request.url))
                    def no_overflow(page, state):
                        if page.viewport_size['width'] < 900:
                            page.wait_for_function('''() => { const el = document.querySelector('.sidebar'); return !el || el.classList.contains('open') || el.getBoundingClientRect().right <= 0; }''')
                        metrics = page.evaluate('''() => ({width:innerWidth, scroll:document.documentElement.scrollWidth})''')
                        assert metrics['scroll'] <= metrics['width'] + 1, (state, metrics)
                    def chrome_is_translated(page, locale, state):
                        if locale == 'ko':
                            return
                        texts = page.evaluate('''() => {
                            const result = []; const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
                            while (walker.nextNode()) { const node = walker.currentNode, el = node.parentElement;
                              if (!el || el.closest('script,style,select,iframe,code,pre,.avatar,.user-monogram,.workspace-monogram,.topbar-account,.profile-photo-preview') || !el.getClientRects().length) continue;
                              const style = getComputedStyle(el); if (style.visibility === 'hidden' || style.display === 'none') continue;
                              result.push(node.textContent);
                            }
                            for (const el of document.querySelectorAll('[aria-label],[placeholder],[title]')) {
                              if (el.getClientRects().length) for (const attr of ['aria-label','placeholder','title']) if(el.hasAttribute(attr)) result.push(el.getAttribute(attr));
                            }
                            return result;
                        }''')
                        residual = []
                        for text in texts:
                            for user in USER_TEXT:
                                text = text.replace(user, '')
                            if re.search('[가-힣]', text):
                                residual.append(text.strip())
                        assert not residual, (locale, state, residual)
                    def choose(page, locale):
                        picker = page.locator('.language-selector [role=combobox]').first
                        expect(picker).to_be_attached()
                        if not picker.is_visible():
                            page.locator('.workspace-topbar button[aria-controls="workspace-navigation"]').click()
                        select_value(picker, locale)
                        expect(page.locator('html')).to_have_attribute('lang', locale)
                    def nav(page, locale, label):
                        if not page.locator('.primary-navigation').count():
                            page.goto(app + '/#library')
                        def primary(source):
                            control = page.locator('.primary-navigation').get_by_role('button', name=catalogs[locale][source], exact=True)
                            if page.viewport_size['width'] < 900 and not page.locator('.sidebar').evaluate("el => el.classList.contains('open')"):
                                page.get_by_role('button', name=catalogs[locale]['메뉴 열기'], exact=True).click()
                            control.click()
                            if page.viewport_size['width'] < 900:
                                page.wait_for_function("() => document.querySelector('.sidebar').getBoundingClientRect().right <= 0")
                        if label == '내 계정':
                            page.get_by_role('button', name=catalogs[locale]['내 계정 열기'], exact=True).click()
                        elif label in ['컬렉션', '에이전트']:
                            primary(label)
                        elif label in ['전체 아티팩트','내 작업','조직 공유함','즐겨찾기','아카이브','휴지통']:
                            primary('라이브러리')
                            page.locator('.library-filters').get_by_role('button', name=catalogs[locale][label], exact=True).click()
                        else:
                            primary('설정')
                            page.locator('.settings-navigation').get_by_role('button', name=catalogs[locale][label], exact=True).click()
                    def open_detail(page, locale):
                        nav(page, locale, '전체 아티팩트')
                        page.locator('.artifact-card').get_by_role('link', name=catalogs[locale]['{p0} 열기'].replace('{p0}', USER_TEXT[0]), exact=True).click()
                        expect(page.locator('.viewer')).to_be_visible()
                        expect(page.locator('.theme-toggle,.language-selector')).to_have_count(0)
                        page.get_by_role('button', name=catalogs[locale]['아티팩트 상세'], exact=True).click()
                        detail = page.get_by_role('dialog')
                        expect(detail.locator('.detail-version-picker [role=combobox]')).to_be_visible()
                        return detail
                    def close_detail(page, locale):
                        page.get_by_role('dialog').get_by_role('button', name=catalogs[locale]['닫기'], exact=True).click()
                        expect(page.get_by_role('dialog')).to_have_count(0)

                    for index, locale in enumerate(LOCALES):
                        ctx = browser.new_context(viewport={'width': 1440, 'height': 1000}, locale='fr-FR')
                        page = ctx.new_page()
                        install_watchers(page)
                        page.goto(app)
                        expect(page.locator('html')).to_have_attribute('lang', 'ko')
                        choose(page, locale)
                        expect(page.get_by_role('heading', name=catalogs[locale]['작업 공간에 로그인'], exact=True)).to_be_visible()
                        page.get_by_label(catalogs[locale]['이메일'], exact=True).fill('i18n@folio.test')
                        page.get_by_label(catalogs[locale]['비밀번호'], exact=True).fill('wrong-fixture-password-123!')
                        page.get_by_role('button', name=catalogs[locale]['로그인'], exact=True).click()
                        expect(page.get_by_role('alert')).to_contain_text(catalogs[locale]['이메일 또는 비밀번호가 올바르지 않습니다.'])
                        # API error objects must react to a language change without resubmitting.
                        other_locale = LOCALES[(index + 1) % len(LOCALES)]
                        choose(page, other_locale)
                        expect(page.get_by_role('alert')).to_contain_text(catalogs[other_locale]['이메일 또는 비밀번호가 올바르지 않습니다.'])
                        choose(page, locale)
                        page.reload()
                        expect(page.locator('html')).to_have_attribute('lang', locale)
                        chrome_is_translated(page, locale, 'login')
                        no_overflow(page, locale + '-login')
                        if locale in ['ja', 'fr']:
                            page.screenshot(path=str(output / f'{locale}-desktop-login.png'), full_page=True)
                        if locale in ['ja', 'zh-CN', 'fr']:
                            page.set_viewport_size({'width': 390, 'height': 844})
                            no_overflow(page, locale + '-mobile-login')
                            page.screenshot(path=str(output / f'{locale}-mobile-login.png'), full_page=True)
                            page.set_viewport_size({'width': 1440, 'height': 1000})
                        page.get_by_label(catalogs[locale]['이메일'], exact=True).fill('i18n@folio.test')
                        page.get_by_label(catalogs[locale]['비밀번호'], exact=True).fill(password)
                        page.get_by_role('button', name=catalogs[locale]['로그인'], exact=True).click()
                        expect(page.get_by_role('heading', name=catalogs[locale]['전체 아티팩트'], exact=True)).to_be_visible()
                        expect(page.locator('.artifact-card')).to_contain_text(USER_TEXT[0])
                        chrome_is_translated(page, locale, 'library')
                        for label in ['구성원', '저장소와 보관', '도메인', '에이전트', '조직 설정', '백업 및 복원', '컬렉션', '즐겨찾기', '휴지통', '내 계정', '변경 기록']:
                            nav(page, locale, label)
                            expect(page.locator('main h1,main h2').first).to_be_visible()
                            chrome_is_translated(page, locale, label)
                            no_overflow(page, locale + '-' + label)
                        nav(page, locale, '컬렉션')
                        expect(page.get_by_text(USER_TEXT[4], exact=True).first).to_be_visible()
                        if locale == 'fr':
                            page.screenshot(path=str(output / 'fr-desktop-collections.png'), full_page=True)
                        detail = open_detail(page, locale)
                        detail.get_by_role('button', name=catalogs[locale]['버전 {p0}'].replace('{p0}', '2'), exact=True).click()
                        expected_month = page.evaluate('(locale) => new Intl.DateTimeFormat(locale,{month:"short"}).format(new Date())', locale)
                        expect(detail.locator('.version-item small').first).to_contain_text(expected_month)
                        for label in ['검토 및 피드백', '버전 비교', '출처']:
                            detail.get_by_role('button', name=catalogs[locale][label], exact=True).click()
                            expect(detail.locator('.review-panel,.version-compare,.artifact-context').first).to_be_visible()
                            if label == '버전 비교':
                                expect(detail.locator('.compare-previews iframe')).to_have_count(2)
                                expect(detail.frame_locator('.compare-previews iframe').first.locator('h1')).to_have_text(USER_TEXT[0] + ' 1')
                                expect(detail.frame_locator('.compare-previews iframe').last.locator('h1')).to_have_text(USER_TEXT[0] + ' 2')
                            chrome_is_translated(page, locale, label)
                            no_overflow(page, locale + '-' + label)
                            if locale in ['ja', 'fr']:
                                page.screenshot(path=str(output / f'{locale}-desktop-{label}.png'), full_page=True)
                        close_detail(page, locale)
                        nav(page, locale, '내 계정')
                        # Client validation errors must remain localized through rerendering.
                        page.get_by_label(catalogs[locale]['현재 비밀번호'], exact=True).fill(password)
                        page.get_by_label(catalogs[locale]['새 비밀번호'], exact=True).fill('New-fixture-password-123!')
                        page.get_by_label(catalogs[locale]['새 비밀번호 확인'], exact=True).fill('Different-fixture-password-123!')
                        page.get_by_role('button', name=catalogs[locale]['비밀번호 변경'], exact=True).click()
                        expect(page.get_by_role('alert')).to_contain_text(catalogs[locale]['새 비밀번호가 일치하지 않습니다.'])
                        choose(page, other_locale)
                        expect(page.get_by_role('alert')).to_contain_text(catalogs[other_locale]['새 비밀번호가 일치하지 않습니다.'])
                        choose(page, locale)
                        # Formatting proof comes from the actual locale-formatted storage meter.
                        nav(page, locale, '전체 아티팩트')
                        meter = page.locator('.capacity-inline').inner_text()
                        assert ('1,5 GB' if locale == 'fr' else '1.5 GB') in meter, meter
                        if locale in ['ja', 'zh-CN', 'fr']:
                            page.set_viewport_size({'width': 390, 'height': 844})
                            no_overflow(page, locale + '-mobile-library')
                            expect(page.frame_locator('.artifact-card iframe').locator('h1')).to_contain_text(USER_TEXT[0])
                            page.screenshot(path=str(output / f'{locale}-mobile-library.png'), full_page=True)
                            nav(page, locale, '백업 및 복원')
                            no_overflow(page, locale + '-mobile-backups')
                            chrome_is_translated(page, locale, 'mobile-backups')
                            page.screenshot(path=str(output / f'{locale}-mobile-backups.png'), full_page=True)
                            page.get_by_role('button', name=catalogs[locale]['메뉴 열기'], exact=True).click()
                            expect(page.locator('.sidebar')).to_have_attribute('role', 'dialog')
                            page.keyboard.press('Tab')
                            assert page.locator('.sidebar').evaluate('(el) => el.contains(document.activeElement)')
                            page.keyboard.press('Escape')
                            expect(page.locator('.sidebar')).not_to_have_class(re.compile('open'))
                            detail = open_detail(page, locale)
                            detail.get_by_role('button', name=catalogs[locale]['버전 비교'], exact=True).click()
                            # On mobile the second preview is intentionally lazy-loaded below the fold.
                            detail.locator('.compare-previews .artifact-preview').last.scroll_into_view_if_needed()
                            expect(detail.locator('.compare-previews iframe')).to_have_count(2)
                            expect(detail.frame_locator('.compare-previews iframe').first.locator('h1')).to_have_text(USER_TEXT[0] + ' 1')
                            expect(detail.frame_locator('.compare-previews iframe').last.locator('h1')).to_have_text(USER_TEXT[0] + ' 2')
                            detail.locator('.version-compare > header').scroll_into_view_if_needed()
                            no_overflow(page, locale + '-mobile-detail')
                            page.screenshot(path=str(output / f'{locale}-mobile-detail.png'), full_page=True)
                            # Dialog focus remains trapped while translated tab names wrap/scroll.
                            page.keyboard.press('Tab')
                            assert detail.evaluate('(el) => el.contains(document.activeElement)')
                            close_detail(page, locale)
                        guest_ctx = browser.new_context(viewport={'width': 390, 'height': 844})
                        guest_page = guest_ctx.new_page()
                        install_watchers(guest_page)
                        guest_page.add_init_script('localStorage.setItem("folio.locale", ' + json.dumps(locale) + ')')
                        guest_page.goto(guest_url)
                        expect(guest_page.get_by_role('heading', name=catalogs[locale]['공유된 아티팩트'], exact=True)).to_be_visible()
                        expect(guest_page.locator('.theme-toggle,.language-selector')).to_have_count(0)
                        chrome_is_translated(guest_page, locale, 'guest-password')
                        no_overflow(guest_page, locale + '-guest')
                        guest_ctx.close()
                        ctx.close()
                        checks.append(locale + ': login persistence, API/client errors, all navigation, review/context/diff, guest, user content')
                        print('PASS ' + checks[-1], flush=True)
                    external = [url for url in requests if not re.match(r'^https?://[^/]*\.localhost:', url)]
                    assert not external, external
                    assert not errors, errors
                    browser.close()
                result = {'status': 'PASS', 'checks': checks, 'pageErrors': errors, 'externalRequests': [], 'screenshots': str(output)}
                (output / 'result.json').write_text(json.dumps(result, ensure_ascii=False, indent=2))
                print(json.dumps(result, ensure_ascii=False, indent=2))
            except Exception:
                try:
                    if 'page' in locals() and not page.is_closed():
                        page.screenshot(path=str(output / 'failure.png'), full_page=True)
                except Exception:
                    pass
                raise
            finally:
                proc.terminate()
                try:
                    proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()


if __name__ == '__main__':
    main()
