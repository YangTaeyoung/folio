#!/usr/bin/env python3
"""Real UI acceptance test. Requires a built Folio binary and Python Playwright.
Run: python web/tests/e2e.py --binary ./dist/folio --screenshots .work/e2e
Uses an isolated temporary installation; never connects to a cloud account.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.request
from playwright.sync_api import sync_playwright, expect


def free_port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', default='./dist/folio')
    parser.add_argument('--screenshots', default='.work/e2e')
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())
    fixture = Path(__file__).parent / 'fixtures/site'
    screenshots = Path(args.screenshots).resolve()
    screenshots.mkdir(parents=True, exist_ok=True)
    app_port, content_port = free_port(), free_port()
    app = f'http://folio.localhost:{app_port}'
    content = f'http://{{artifact}}.content.folio.localhost:{content_port}'
    password = 'Folio-UI-test-only-123!'
    errors = []
    with tempfile.TemporaryDirectory(prefix='folio-browser-') as data:
        env = {**os.environ, 'FOLIO_BOOTSTRAP_TOKEN': 'browser-test-setup'}
        log = open(Path(data) / 'server.log', 'w+')
        proc = subprocess.Popen([binary, 'serve', '--data-dir', data,
            '--listen', f'127.0.0.1:{app_port}', '--content-listen', f'127.0.0.1:{content_port}',
            '--app-url', app, '--content-url', content], env=env, stdout=log, stderr=log)
        try:
            for _ in range(100):
                if proc.poll() is not None:
                    log.seek(0)
                    raise RuntimeError(log.read())
                try:
                    with urllib.request.urlopen(f'http://127.0.0.1:{app_port}/api/bootstrap', timeout=.5) as r:
                        # Host validation intentionally rejects loopback instead of advertised host.
                        pass
                except urllib.error.HTTPError:
                    break
                except OSError:
                    time.sleep(.1)
            with sync_playwright() as p:
                browser = p.chromium.launch(headless=True)
                admin_ctx = browser.new_context(viewport={'width':1440, 'height':1000})
                page = admin_ctx.new_page()
                page.on('pageerror', lambda e: errors.append(str(e)))
                page.goto(app + '/#setup=browser-test-setup')
                page.wait_for_load_state('networkidle')
                page.get_by_label('조직 이름', exact=True).fill('Folio Test Studio')
                page.get_by_label('이름', exact=True).fill('Admin')
                page.get_by_label('이메일', exact=True).fill('admin@folio.test')
                page.get_by_label('비밀번호', exact=True).fill(password)
                page.get_by_role('button', name='작업 공간 만들기', exact=True).click()
                expect(page.get_by_role('heading', name='전체 아티팩트')).to_be_visible()
                page.get_by_role('button', name='구성원', exact=True).click()

                def create_member(name, email):
                    page.get_by_role('button', name='구성원 추가', exact=True).click()
                    modal = page.get_by_role('dialog')
                    modal.get_by_label('이름', exact=True).fill(name)
                    modal.get_by_label('이메일', exact=True).fill(email)
                    modal.get_by_role('button', name='계정 만들기', exact=True).click()
                    expect(modal).to_have_count(0)
                    expect(page.locator('.secret-box code')).to_be_visible()
                    temporary = page.locator('.secret-box code').inner_text()
                    page.get_by_role('button', name='비밀번호 표시 닫기').click()
                    return temporary

                expect(page.get_by_role('row').filter(has_text='admin@folio.test').get_by_role('button', name='초기화', exact=True)).to_be_disabled()
                alice_temp = create_member('Alice', 'alice@folio.test')
                bob_temp = create_member('Bob', 'bob@folio.test')

                def member(name, email, temporary):
                    ctx = browser.new_context()
                    q = ctx.new_page()
                    q.on('pageerror', lambda e: errors.append(str(e)))
                    q.goto(app)
                    q.get_by_label('이메일', exact=True).fill(email)
                    q.get_by_label('비밀번호', exact=True).fill(temporary)
                    q.get_by_role('button', name='비밀번호 표시', exact=True).click()
                    expect(q.get_by_label('비밀번호', exact=True)).to_have_attribute('type', 'text')
                    expect(q.get_by_label('비밀번호', exact=True)).to_have_value(temporary)
                    q.get_by_role('button', name='비밀번호 숨기기', exact=True).click()
                    expect(q.get_by_label('비밀번호', exact=True)).to_have_attribute('type', 'password')
                    q.get_by_role('button', name='로그인', exact=True).click()
                    expect(q.get_by_role('heading', name='나만의 비밀번호로 바꿔주세요')).to_be_visible()
                    q.get_by_label('임시 비밀번호', exact=True).fill(temporary)
                    q.get_by_label('새 비밀번호 · 12자 이상', exact=True).fill(password)
                    q.get_by_label('새 비밀번호 확인', exact=True).fill(password)
                    q.get_by_role('button', name='변경하고 시작하기').click()
                    expect(q.get_by_role('heading', name='전체 아티팩트')).to_be_visible()
                    expect(q.get_by_role('button', name='구성원', exact=True)).to_have_count(0)
                    return ctx, q

                alice_ctx, alice = member('Alice', 'alice@folio.test', alice_temp)
                bob_ctx, bob = member('Bob', 'bob@folio.test', bob_temp)
                page.get_by_role('button', name='전체 아티팩트', exact=True).click()
                page.get_by_role('button', name='새 아티팩트', exact=True).first.click()
                modal = page.get_by_role('dialog')
                modal.locator('input[webkitdirectory]').set_input_files(str(fixture))
                modal.get_by_label('제목', exact=True).fill('Working prototype')
                modal.get_by_label('설명', exact=True).fill('Published through the actual Folio interface.')
                modal.get_by_role('button', name='아티팩트 게시', exact=True).click()
                expect(modal).to_have_count(0)
                expect(page.get_by_role('button', name='Working prototype 열기')).to_be_visible()
                page.get_by_role('button', name='Working prototype 열기').click()
                detail = page.get_by_role('dialog')
                frame = detail.frame_locator('iframe')
                expect(frame.locator('#status')).to_have_text('Relative ES modules ready')
                expect(frame.locator('#worker-status')).to_have_text('Dedicated worker ready')
                expect(frame.locator('#service-worker-status')).to_have_text('Service worker rejected')
                frame.get_by_role('button', name='Count: 0', exact=True).click()
                expect(frame.get_by_role('button', name='Count: 1', exact=True)).to_be_visible()
                page.screenshot(path=str(screenshots / 'detail-desktop.png'), full_page=True)
                detail.get_by_role('button', name='공유', exact=True).click()
                share = page.get_by_role('dialog')
                link = share.get_by_label('공유 링크').input_value()
                share.get_by_role('radio', name='선택한 구성원', exact=False).check()
                share.locator('.person-option').filter(has_text='alice@folio.test').get_by_role('checkbox').check()
                page.screenshot(path=str(screenshots / 'sharing-desktop.png'), full_page=True)
                share.get_by_role('button', name='공유 설정 저장').click()
                expect(share).to_have_count(0)
                alice.goto(link)
                expect(alice.frame_locator('iframe').locator('#status')).to_have_text('Relative ES modules ready')
                bob.goto(link)
                expect(bob.get_by_text('이 아티팩트를 열 수 없습니다', exact=True)).to_be_visible()
                anonymous_ctx = browser.new_context()
                anonymous = anonymous_ctx.new_page()
                anonymous.goto(link)
                expect(anonymous.get_by_text('이 아티팩트를 열 수 없습니다', exact=True)).to_be_visible()
                page.get_by_role('button', name='Working prototype 공유').click()
                share = page.get_by_role('dialog')
                share.get_by_role('radio', name='조직 구성원', exact=False).check()
                share.get_by_role('button', name='공유 설정 저장').click()
                bob.reload()
                expect(bob.frame_locator('iframe').locator('#status')).to_have_text('Relative ES modules ready')
                anonymous.reload()
                expect(anonymous.get_by_text('이 아티팩트를 열 수 없습니다', exact=True)).to_be_visible()
                page.get_by_role('button', name='Working prototype 공유').click()
                share = page.get_by_role('dialog')
                share.get_by_role('radio', name='링크가 있는 모든 사람', exact=False).check()
                share.get_by_role('button', name='공유 설정 저장').click()
                anonymous.reload()
                expect(anonymous.frame_locator('iframe').locator('#status')).to_have_text('Relative ES modules ready')
                # Existing iframe must also lose its live server access after revocation.
                anonymous.frame_locator('iframe').get_by_role('button', name='Count: 0', exact=True).click()
                page.get_by_role('button', name='Working prototype 공유').click()
                share = page.get_by_role('dialog')
                share.get_by_role('radio', name='나만 보기', exact=False).check()
                share.get_by_role('button', name='공유 설정 저장').click()
                anonymous.reload()
                expect(anonymous.get_by_text('이 아티팩트를 열 수 없습니다', exact=True)).to_be_visible()
                # New version and restore, using actual upload UI.
                page.get_by_role('button', name='Working prototype 열기').click()
                page.get_by_role('dialog').get_by_role('button', name='새 버전', exact=True).click()
                next_site = Path(data) / 'revision-two'
                shutil.copytree(fixture, next_site)
                js = next_site / 'app.js'
                js.write_text(js.read_text().replace('Relative ES modules ready', 'Revision two ready'))
                modal = page.get_by_role('dialog')
                modal.locator('input[webkitdirectory]').set_input_files(str(next_site))
                modal.get_by_label('버전 메모', exact=True).fill('Second revision')
                modal.get_by_role('button', name='새 버전 게시', exact=True).click()
                expect(modal).to_have_count(0)
                page.get_by_role('button', name='Working prototype 열기').click()
                detail = page.get_by_role('dialog')
                expect(detail.frame_locator('iframe').locator('#status')).to_have_text('Revision two ready')
                detail.get_by_role('button', name='버전 2', exact=True).click()
                detail.get_by_role('button', name='복원', exact=True).click()
                detail.get_by_role('button', name='확인', exact=True).click()
                detail.get_by_role('button', name='미리보기', exact=True).click()
                expect(detail.frame_locator('iframe').locator('#status')).to_have_text('Relative ES modules ready')
                with page.expect_download() as download:
                    detail.get_by_role('link', name='다운로드', exact=True).click()
                assert download.value.suggested_filename.endswith('.zip')
                detail.get_by_role('button', name='아카이브', exact=True).click()
                expect(page.get_by_role('dialog')).to_have_count(0)
                page.get_by_role('button', name='아카이브', exact=True).click()
                expect(page.get_by_role('button', name='Working prototype 열기')).to_be_visible()
                page.get_by_role('button', name='Working prototype 열기').click()
                page.get_by_role('dialog').get_by_role('button', name='아카이브에서 복원', exact=True).click()
                page.get_by_role('button', name='전체 아티팩트', exact=True).click()
                expect(page.get_by_role('button', name='Working prototype 열기')).to_be_visible()
                # Every administration screen must render without runtime errors and horizontal overflow.
                for nav in ['구성원', '저장소와 보관', '도메인', '에이전트', '조직 설정', '변경 기록']:
                    page.get_by_role('button', name=nav, exact=True).click()
                    page.wait_for_load_state('networkidle')
                    assert page.get_by_role('heading', level=1).count() == 1, nav
                    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), nav
                page.get_by_role('button', name='저장소와 보관', exact=True).click()
                page.get_by_role('button', name='연결 테스트', exact=True).click()
                expect(page.get_by_text('Local storage: 쓰기·읽기·삭제 테스트를 통과했습니다.', exact=True)).to_be_visible()
                page.get_by_role('button', name='정리 대상 미리보기', exact=True).click()
                expect(page.get_by_role('dialog').get_by_text('현재 정리할 대상이 없습니다.', exact=True)).to_be_visible()
                page.get_by_role('dialog').get_by_role('button', name='닫기', exact=True).first.click()
                page.get_by_role('button', name='전체 아티팩트', exact=True).click()
                page.screenshot(path=str(screenshots / 'library-desktop.png'), full_page=True)
                page.set_viewport_size({'width':390,'height':844})
                expect(page.get_by_role('button', name='메뉴 열기')).to_be_visible()
                assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
                page.get_by_role('button', name='Working prototype 공유').click()
                expect(page.get_by_role('dialog')).to_be_visible()
                assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
                expect(page.get_by_role('dialog')).to_have_css('width','390px')
                page.wait_for_timeout(250)
                page.screenshot(path=str(screenshots / 'sharing-mobile.png'), full_page=True)
                page.get_by_role('button', name='닫기', exact=True).click()
                expect(page.locator('.sidebar')).to_have_attribute('inert', '')
                menu = page.get_by_role('button', name='메뉴 열기')
                menu.click()
                navigation = page.get_by_role('dialog', name='작업 공간 탐색')
                expect(navigation.get_by_role('button', name='메뉴 닫기')).to_be_focused()
                first_nav = navigation.locator('a[href]').first
                last_nav = navigation.get_by_role('button', name='로그아웃', exact=True)
                last_nav.focus()
                page.keyboard.press('Tab')
                expect(first_nav).to_be_focused()
                page.keyboard.press('Shift+Tab')
                expect(last_nav).to_be_focused()
                page.keyboard.press('Escape')
                expect(page.locator('.sidebar')).to_have_attribute('inert', '')
                expect(menu).to_be_focused()
                menu.click()
                navigation.get_by_role('button', name='메뉴 닫기').click()
                expect(menu).to_be_focused()
                menu.click()
                expect(page.get_by_role('button', name='내 아티팩트', exact=True)).to_be_visible()
                page.screenshot(path=str(screenshots / 'navigation-mobile.png'), full_page=True)
                page.get_by_role('button', name='내 계정', exact=True).click()
                page.get_by_label('현재 비밀번호', exact=True).fill(password)
                next_password = 'Folio-updated-password-123!'
                page.get_by_label('새 비밀번호', exact=True).fill(next_password)
                page.get_by_label('새 비밀번호 확인', exact=True).fill(next_password)
                page.get_by_role('button', name='비밀번호 변경', exact=True).click()
                expect(page.get_by_text('비밀번호를 변경했습니다.', exact=True)).to_be_visible()
                page.set_viewport_size({'width':1440,'height':1000})
                page.get_by_role('button', name='전체 아티팩트', exact=True).click()
                # A long library must acquire a content grant when scrolled into view,
                # rather than spending its 60-second lifetime below the fold.
                page.evaluate('''async()=>{const me=await(await fetch('/api/me')).json();for(let i=0;i<18;i++){const f=new FormData();f.append('metadata',JSON.stringify({title:'Scroll fixture '+i,files:[{path:'index.html'}]}));f.append('file0',new Blob(['<!doctype html><title>Scroll fixture</title><h1>Scroll preview ready</h1>']),'index.html');const r=await fetch('/api/artifacts',{method:'POST',headers:{'X-CSRF-Token':me.csrfToken},body:f});if(!r.ok)throw new Error('Fixture publication failed')}}''')
                page.reload(); page.wait_for_load_state('networkidle')
                last = page.locator('.artifact-card').last
                expect(last.locator('iframe')).to_have_count(0)
                last.scroll_into_view_if_needed()
                expect(last.locator('iframe')).to_have_count(1)
                expect(last.frame_locator('iframe').locator('#status')).to_have_text('Relative ES modules ready')
                page.get_by_role('button', name='로그아웃', exact=True).click()
                expect(page.get_by_role('heading', name='작업 공간에 로그인')).to_be_visible()
                page.get_by_label('이메일', exact=True).fill('admin@folio.test')
                page.get_by_label('비밀번호', exact=True).fill(next_password)
                page.get_by_role('button', name='로그인', exact=True).click()
                expect(page.get_by_role('heading', name='전체 아티팩트')).to_be_visible()
                assert not errors, errors
                browser.close()
                print(json.dumps({'status':'PASS','checks':['bootstrap','admin-created-members','forced-password-change','relative-es-modules','selected-share','organization-share','public-share','revocation','versions-and-rollback','zip-download','archive-restore','admin-screens','local-storage-probe','cleanup-preview','responsive-mobile','self-reset-guard','normal-password-change','below-fold-preview-grants','dedicated-worker','service-worker-rejected'],'pageErrors':errors,'screenshots':str(screenshots)}))
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()
            log.close()

if __name__ == '__main__':
    main()
