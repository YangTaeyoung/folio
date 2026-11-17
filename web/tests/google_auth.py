#!/usr/bin/env python3
"""Optional Google sign-in UI acceptance against an isolated real Folio process.

No Google account, real OAuth credential, or external browser request is used.
The authorization redirect is inspected with redirects disabled, never followed.
Run: .work/py/bin/python web/tests/google_auth.py --binary .work/auth-visuals/folio
"""
import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

from browser_controls import choose
from playwright.sync_api import expect, sync_playwright


def free_port():
    with socket.socket() as connection:
        connection.bind(('127.0.0.1', 0))
        return connection.getsockname()[1]


def no_overflow(page, label):
    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), label


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', default='.work/auth-visuals/folio')
    parser.add_argument('--screenshots', default='.work/auth-visuals/google-captures')
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())
    screenshots = Path(args.screenshots).resolve()
    screenshots.mkdir(parents=True, exist_ok=True)
    app_port, content_port = free_port(), free_port()
    app = f'http://folio.localhost:{app_port}'
    content = f'http://{{artifact}}.content.folio.localhost:{content_port}'
    password = 'Folio-Google-UI-test-only-123!'
    client_id = 'folio-browser-fixture.apps.googleusercontent.com'
    fake_secret = 'isolated-browser-test-not-a-real-google-secret'
    errors, external_requests = [], []
    checks = []
    with tempfile.TemporaryDirectory(prefix='folio-google-browser-') as data:
        env = {**os.environ, 'FOLIO_BOOTSTRAP_TOKEN': 'google-browser-test-setup'}
        with open(Path(data) / 'server.log', 'w+') as log:
            proc = subprocess.Popen([
                binary, 'serve', '--data-dir', data,
                '--listen', f'127.0.0.1:{app_port}',
                '--content-listen', f'127.0.0.1:{content_port}',
                '--app-url', app, '--content-url', content,
            ], env=env, stdout=log, stderr=log)
            try:
                for _ in range(100):
                    if proc.poll() is not None:
                        log.seek(0)
                        raise RuntimeError(log.read())
                    try:
                        urllib.request.urlopen(f'http://127.0.0.1:{app_port}/api/bootstrap', timeout=.5).close()
                    except urllib.error.HTTPError:
                        break  # Expected host validation proves the listener is ready.
                    except OSError:
                        time.sleep(.1)
                else:
                    raise RuntimeError('Isolated Folio listener did not become ready')

                with sync_playwright() as playwright:
                    browser = playwright.chromium.launch(headless=True)
                    context = browser.new_context(viewport={'width': 1440, 'height': 1080})

                    def local_only(route):
                        host = urllib.parse.urlparse(route.request.url).hostname
                        if host != 'folio.localhost':
                            external_requests.append(route.request.url)
                            route.abort()
                        else:
                            route.continue_()

                    context.route('**/*', local_only)
                    page = context.new_page()
                    page.on('pageerror', lambda error: errors.append(str(error)))
                    providers = context.request.get(app + '/api/auth/providers')
                    assert providers.status == 200
                    assert providers.json()['google']['enabled'] is False
                    page.goto(app + '/#setup=google-browser-test-setup')
                    page.get_by_label('조직 이름', exact=True).fill('Google Browser Fixture')
                    page.get_by_label('이름', exact=True).fill('Fixture Admin')
                    page.get_by_label('이메일', exact=True).fill('admin@folio.test')
                    page.get_by_label('비밀번호', exact=True).fill(password)
                    page.get_by_role('button', name='작업 공간 만들기', exact=True).click()
                    expect(page.get_by_role('heading', name='전체 아티팩트', exact=True)).to_be_visible()
                    checks.append('real-isolated-bootstrap')

                    # Default provider must not add a Google CTA or contact Google.
                    page.get_by_role('button', name='로그아웃', exact=True).click()
                    expect(page.get_by_role('heading', name='작업 공간에 로그인')).to_be_visible()
                    expect(page.get_by_role('link', name='Google로 로그인', exact=True)).to_have_count(0)
                    page.get_by_label('이메일', exact=True).fill('admin@folio.test')
                    page.get_by_label('비밀번호', exact=True).fill(password)
                    page.get_by_role('button', name='로그인', exact=True).click()
                    expect(page.get_by_role('heading', name='전체 아티팩트', exact=True)).to_be_visible()
                    checks.append('google-default-off-local-password-login')

                    page.get_by_role('button', name='조직 설정', exact=True).click()
                    settings = page.locator('.google-auth-settings')
                    expect(settings.get_by_label('Google 로그인 사용', exact=True)).not_to_be_checked()
                    expect(settings.get_by_label('승인된 리디렉션 URI', exact=True)).to_have_value(app + '/api/auth/google/callback')
                    expect(page.get_by_role('heading', level=1)).to_have_count(1)
                    settings.get_by_label('Client ID', exact=True).fill(client_id)
                    settings.get_by_label('Client Secret', exact=True).fill(fake_secret)
                    settings.get_by_label('허용할 Google Workspace 도메인 (선택)', exact=True).fill('example.com')
                    settings.get_by_label('Google 로그인 사용', exact=True).check()
                    with page.expect_response(lambda response: response.url == app + '/api/admin/auth/google' and response.request.method == 'PATCH') as first_save:
                        settings.get_by_role('button', name='Google 로그인 설정 저장', exact=True).click()
                    assert first_save.value.status == 200, first_save.value.text()
                    config = first_save.value.json()
                    assert config['enabled'] and config['clientSecretConfigured']
                    assert config['allowedDomains'] == ['example.com']
                    assert 'clientSecret' not in config and fake_secret not in first_save.value.text()
                    expect(settings.get_by_role('status')).to_have_text('저장했습니다.')
                    expect(settings.get_by_label('Client Secret', exact=True)).to_have_value('')
                    checks.append('real-admin-patch-enabled-redacted-secret-and-domains')

                    # A second real UI save omits the empty secret and preserves it.
                    settings.get_by_label('허용할 Google Workspace 도메인 (선택)', exact=True).fill('')
                    expect(settings.get_by_role('status')).to_have_count(0)
                    with page.expect_response(lambda response: response.url == app + '/api/admin/auth/google' and response.request.method == 'PATCH') as second_save:
                        settings.get_by_role('button', name='Google 로그인 설정 저장', exact=True).click()
                    assert second_save.value.status == 200, second_save.value.text()
                    assert 'clientSecret' not in second_save.value.request.post_data_json
                    assert second_save.value.json()['clientSecretConfigured'] is True
                    assert second_save.value.json()['allowedDomains'] == []
                    expect(settings.get_by_role('status')).to_have_text('저장했습니다.')
                    page.reload()
                    expect(settings.get_by_label('Client Secret', exact=True)).to_have_value('')
                    expect(settings.get_by_label('Client Secret', exact=True)).to_have_attribute('placeholder', '저장된 비밀 키 유지')
                    expect(settings.get_by_label('Google 로그인 사용', exact=True)).to_be_checked()
                    checks.append('real-ui-blank-secret-retains-config-after-reload')

                    no_overflow(page, 'settings desktop')
                    page.screenshot(path=str(screenshots / 'settings-desktop.png'), full_page=True)
                    page.set_viewport_size({'width': 390, 'height': 844})
                    expect(page.locator('.sidebar')).to_have_attribute('inert', '')
                    page.wait_for_function("() => document.querySelector('.sidebar').getBoundingClientRect().right <= 0")
                    no_overflow(page, 'settings mobile')
                    expect(page.get_by_role('heading', level=1)).to_have_count(1)
                    page.screenshot(path=str(screenshots / 'settings-mobile.png'), full_page=True)
                    page.set_viewport_size({'width': 1440, 'height': 1080})

                    page.get_by_role('button', name='구성원', exact=True).click()
                    page.get_by_role('button', name='구성원 추가', exact=True).click()
                    modal = page.get_by_role('dialog')
                    expect(modal.get_by_label('로그인 방법', exact=True)).to_have_attribute('data-value', 'password')
                    modal.get_by_label('이름', exact=True).fill('Google Fixture')
                    modal.get_by_label('이메일', exact=True).fill('google.fixture@gmail.com')
                    choose(modal.get_by_label('로그인 방법', exact=True), 'google')
                    expect(modal.get_by_text('임시 비밀번호는 생성하지 않습니다.', exact=False)).to_be_visible()
                    with page.expect_response(lambda response: response.url == app + '/api/admin/members' and response.request.method == 'POST') as created:
                        modal.get_by_role('button', name='계정 만들기', exact=True).click()
                    assert created.value.status in (200, 201), created.value.text()
                    assert not created.value.json().get('temporaryPassword')
                    assert created.value.request.post_data_json['loginMethod'] == 'google'
                    expect(modal).to_have_count(0)
                    row = page.get_by_role('row').filter(has_text='google.fixture@gmail.com')
                    expect(row).to_be_visible()
                    expect(row.get_by_text('Google 로그인', exact=True)).to_be_visible()
                    expect(row.get_by_role('button', name='초기화', exact=True)).to_have_count(0)
                    expect(page.locator('.secret-box')).to_have_count(0)
                    expect(page.get_by_role('row').filter(has_text='admin@folio.test').get_by_role('button', name='초기화', exact=True)).to_be_disabled()
                    checks.append('real-google-only-provision-no-temporary-password-no-reset')

                    page.get_by_role('button', name='로그아웃', exact=True).click()
                    cta = page.get_by_role('link', name='Google로 로그인', exact=True)
                    expect(cta).to_be_visible()
                    expect(cta).to_have_attribute('href', '/api/auth/google/start')
                    no_overflow(page, 'Google enabled login desktop')
                    page.screenshot(path=str(screenshots / 'login-desktop.png'), full_page=True)
                    page.set_viewport_size({'width': 390, 'height': 844})
                    expect(cta).to_be_visible()
                    no_overflow(page, 'Google enabled login mobile')
                    page.screenshot(path=str(screenshots / 'login-mobile.png'), full_page=True)

                    # Follow only the local start endpoint, never Google's Location.
                    start = context.request.get(app + '/api/auth/google/start', max_redirects=0)
                    assert start.status == 303
                    target = urllib.parse.urlparse(start.headers['location'])
                    query = urllib.parse.parse_qs(target.query)
                    assert target.scheme == 'https' and target.hostname == 'accounts.google.com'
                    assert query['client_id'] == [client_id]
                    assert query['redirect_uri'] == [app + '/api/auth/google/callback']
                    assert query['response_type'] == ['code']
                    assert set(query['scope'][0].split()) == {'openid', 'email'}
                    assert query['code_challenge_method'] == ['S256']
                    assert len(query['code_challenge'][0]) == 43
                    assert len(query['state'][0]) >= 32 and len(query['nonce'][0]) >= 32
                    assert 'client_secret' not in query and fake_secret not in start.headers['location']
                    assert 'HttpOnly' in start.headers['set-cookie']
                    checks.append('local-authorization-start-state-nonce-pkce-redirect-inspected-without-google')

                    # Error query contents are not rendered or interpreted as markup.
                    hostile = '<img src=x onerror=alert(1)>'
                    page.goto(app + '/?auth_error=' + urllib.parse.quote(hostile, safe=''))
                    expect(page.get_by_role('alert')).to_have_text('Google 로그인에 실패했습니다. 등록된 계정인지 확인하거나 관리자에게 문의하세요.')
                    expect(page.get_by_role('link', name='Google로 로그인', exact=True)).to_be_visible()
                    assert hostile not in page.locator('body').inner_text()
                    expect(page.locator('img[src="x"]')).to_have_count(0)
                    no_overflow(page, 'Google login error mobile')
                    page.screenshot(path=str(screenshots / 'login-error-mobile.png'), full_page=True)
                    checks.extend(['safe-auth-error-visible', 'settings-and-login-desktop-mobile-no-overflow'])
                    assert not errors, errors
                    assert not external_requests, external_requests
                    checks.append('zero-external-browser-requests')
                    browser.close()
                    print(json.dumps({'status': 'PASS', 'checks': checks, 'pageErrors': errors,
                                      'externalBrowserRequests': external_requests, 'screenshots': str(screenshots)}))
            finally:
                proc.terminate()
                try:
                    proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()


if __name__ == '__main__':
    main()
