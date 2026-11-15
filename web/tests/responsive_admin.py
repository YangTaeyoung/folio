#!/usr/bin/env python3
"""Behavioral admin layout checks against an isolated built Folio installation.
Run: python web/tests/responsive_admin.py --binary .work/folio --screenshots .work/responsive-admin
--diagnose captures failures and geometry without failing the process.
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
import urllib.request
from playwright.sync_api import expect, sync_playwright


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--screenshots', default='.work/responsive-admin')
    parser.add_argument('--diagnose', action='store_true')
    args = parser.parse_args()
    output = Path(args.screenshots).resolve()
    output.mkdir(parents=True, exist_ok=True)
    app_port, content_port = free_port(), free_port()
    app = f'http://folio.localhost:{app_port}'
    failures, results, errors = [], [], []
    with tempfile.TemporaryDirectory(prefix='folio-responsive-admin-') as data:
        with open(Path(data) / 'server.log', 'w+') as log:
            process = subprocess.Popen([
                str(Path(args.binary).resolve()), 'serve', '--data-dir', data,
                '--listen', f'127.0.0.1:{app_port}', '--content-listen', f'127.0.0.1:{content_port}',
                '--app-url', app, '--content-url', f'http://{{artifact}}.content.folio.localhost:{content_port}',
            ], env={**os.environ, 'FOLIO_BOOTSTRAP_TOKEN': 'responsive-admin-test'}, stdout=log, stderr=log)
            try:
                for _ in range(100):
                    if process.poll() is not None:
                        log.seek(0)
                        raise RuntimeError(log.read())
                    try:
                        urllib.request.urlopen(f'http://127.0.0.1:{app_port}/api/bootstrap', timeout=.3)
                    except urllib.error.HTTPError:
                        break
                    except OSError:
                        time.sleep(.1)
                with sync_playwright() as playwright:
                    browser = playwright.chromium.launch(headless=True)
                    context = browser.new_context(viewport={'width': 1440, 'height': 1000})
                    page = context.new_page()
                    page.on('pageerror', lambda error: errors.append(str(error)))
                    page.goto(app + '/#setup=responsive-admin-test')
                    page.get_by_label('조직 이름', exact=True).fill('Responsive Fixture Studio')
                    page.get_by_label('이름', exact=True).fill('Fixture Admin')
                    page.get_by_label('이메일', exact=True).fill('admin@folio.test')
                    page.get_by_label('비밀번호', exact=True).fill('Responsive-Fixture-Only-123!')
                    page.get_by_role('button', name='작업 공간 만들기', exact=True).click()
                    expect(page.get_by_role('heading', name='전체 아티팩트')).to_be_visible()
                    csrf = context.request.get(app + '/api/me').json()['csrfToken']
                    response = context.request.post(app + '/api/admin/members', data={
                        'name': 'Fixture Collaborator With A Longer Name',
                        'email': 'collaborator.long.email@folio.test',
                        'role': 'member', 'loginMethod': 'password',
                    }, headers={'Origin': app, 'X-CSRF-Token': csrf})
                    assert response.ok, response.text()

                    def check(condition, description):
                        if not condition:
                            failures.append(description)

                    def nav(label):
                        if page.viewport_size['width'] <= 900:
                            page.get_by_role('button', name='메뉴 열기', exact=True).click()
                        page.get_by_role('button', name=label, exact=True).click()
                        expect(page.get_by_role('heading', name=label, exact=True)).to_be_visible()
                        page.evaluate('document.fonts.ready')
                        if page.viewport_size['width'] <= 900:
                            expect(page.locator('.sidebar')).to_have_attribute('inert', '')
                            for _ in range(40):
                                sidebar = page.locator('.sidebar').bounding_box()
                                if sidebar['x'] + sidebar['width'] <= 0:
                                    break
                                page.wait_for_timeout(25)
                            else:
                                raise AssertionError('mobile navigation did not finish closing')

                    def capture(name):
                        page.screenshot(path=str(output / (name + '.png')), full_page=False)
                        geometry = page.evaluate('''() => ({viewport:{width:innerWidth,height:innerHeight}, documentWidth:document.documentElement.scrollWidth,
                          elements:Array.from(document.querySelectorAll('.members-panel,.table-scroll,table,th,td,.storage-summary,.storage-summary > div:nth-child(2),[role=dialog],.modal-body,.dialog-footer,[role=dialog] form')).map(e=>{let r=e.getBoundingClientRect(),s=getComputedStyle(e);return {tag:e.tagName,class:e.className,x:r.x,y:r.y,width:r.width,height:r.height,clientHeight:e.clientHeight,scrollHeight:e.scrollHeight,overflowX:s.overflowX,overflowY:s.overflowY,minHeight:s.minHeight,flex:s.flex}})})''')
                        results.append({'name': name, **geometry})
                        print(name, flush=True)

                    def inside_viewport(locator):
                        return locator.evaluate('''e=>{const r=e.getBoundingClientRect();return r.left>=0 && r.right<=innerWidth && r.top>=0 && r.bottom<=innerHeight}''')

                    for width in [1440, 390, 320]:
                        page.set_viewport_size({'width': width, 'height': 1000 if width == 1440 else 844})
                        nav('구성원')
                        row = page.get_by_role('row').filter(has_text='collaborator.long.email@folio.test')
                        expect(row).to_be_visible()
                        row.scroll_into_view_if_needed()
                        capture(f'members-{width}')
                        if args.diagnose and width == 390:
                            print('overflow hypothesis', page.locator('.table-scroll').evaluate('''e=>{const span=e.querySelector('.sr-only');let before={document:document.documentElement.scrollWidth,spanX:span.getBoundingClientRect().x};e.style.position='relative';const after=document.documentElement.scrollWidth;e.style.position='';return {before,afterPositioningScrollWrapper:after}}'''), flush=True)
                        check(page.evaluate('document.documentElement.scrollWidth <= innerWidth'), f'members {width}: document overflows')
                        check(inside_viewport(row.get_by_role('button', name='중지', exact=True)), f'members {width}: suspend action is outside viewport')
                        check(inside_viewport(row.get_by_role('button', name='초기화', exact=True)), f'members {width}: password reset action is outside viewport')
                        check(inside_viewport(row.get_by_text('첫 로그인 대기', exact=True)), f'members {width}: status is outside viewport')
                        if width == 390 and not args.diagnose:
                            row.get_by_role('button', name='중지', exact=True).click()
                            expect(row.get_by_text('일시 중지', exact=True)).to_be_visible()
                            expect(row.get_by_role('button', name='다시 활성화', exact=True)).to_be_visible()
                            row.get_by_role('button', name='다시 활성화', exact=True).click()
                            expect(row.get_by_text('첫 로그인 대기', exact=True)).to_be_visible()

                        nav('저장소와 보관')
                        summary = page.locator('.storage-summary')
                        expect(summary.get_by_text('사용 중인 용량', exact=True)).to_be_visible()
                        capture(f'storage-{width}')
                        check(page.evaluate('document.documentElement.scrollWidth <= innerWidth'), f'storage {width}: document overflows')
                        if width < 600:
                            width_used = summary.locator(':scope > div').nth(1).bounding_box()['width']
                            check(width_used >= 200, f'storage {width}: usage text has only {width_used:.1f}px width')
                        page.get_by_role('button', name='저장소 연결', exact=True).first.click()
                        dialog = page.get_by_role('dialog', name='저장소 연결', exact=True)
                        for provider in ['aws', 'r2', 'gcs', 'azure']:
                            dialog.get_by_label('저장소 종류', exact=True).select_option(provider)
                            dialog.locator('.modal-body').evaluate('e => e.scrollTop = 0')
                            capture(f'provider-{provider}-{width}')
                            submit = dialog.get_by_role('button', name='저장소 연결', exact=True)
                            check(inside_viewport(submit), f'{provider} {width}: submit footer is outside viewport')
                            if not args.diagnose:
                                # The last credential/notice must be reachable without moving the footer.
                                dialog.locator('.notice').scroll_into_view_if_needed()
                                check(inside_viewport(submit), f'{provider} {width}: scrolling fields moves submit out of viewport')
                                check(inside_viewport(dialog.locator('.notice')), f'{provider} {width}: credential notice is unreachable')
                                capture(f'provider-{provider}-{width}-scrolled')
                        dialog.get_by_role('button', name='취소', exact=True).click()
                        expect(dialog).to_have_count(0)
                    check(not errors, 'uncaught JavaScript errors: ' + repr(errors))
                    browser.close()
            finally:
                process.terminate()
                process.wait(timeout=15)
    (output / 'results.json').write_text(json.dumps({'failures': failures, 'errors': errors, 'captures': results}, ensure_ascii=False, indent=2))
    print(json.dumps({'failures': failures, 'errors': errors}, ensure_ascii=False, indent=2))
    if failures and not args.diagnose:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
