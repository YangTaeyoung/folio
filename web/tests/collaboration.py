#!/usr/bin/env python3
"""Collaboration acceptance on a disposable installation, never production.
Run: python web/tests/collaboration.py --binary .work/collaboration/folio --screenshots .work/collaboration/browser
"""
import argparse
import datetime
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import re
import time
from browser_controls import choose
from playwright.sync_api import sync_playwright, expect


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', default='.work/collaboration/folio')
    parser.add_argument('--screenshots', default='.work/collaboration/browser')
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())
    shots = Path(args.screenshots).resolve()
    shots.mkdir(parents=True, exist_ok=True)
    app_port, content_port = free_port(), free_port()
    app = f'http://folio.localhost:{app_port}'
    errors, checks = [], []
    phase = "setup"
    def console_failure(message):
        if message.type != "error":
            return
        location = dict(message.location)
        location["url"] = re.sub(r"/grant/[^/?#]+", "/grant/[redacted]", location.get("url", ""))
        errors.append({"message": message.text, "location": location, "phase": phase})
    with tempfile.TemporaryDirectory(prefix='folio-collaboration-') as data:
        with open(Path(data) / 'server.log', 'w+') as log:
            proc = subprocess.Popen([binary, 'serve', '--data-dir', data,
                '--listen', f'127.0.0.1:{app_port}', '--content-listen', f'127.0.0.1:{content_port}',
                '--app-url', app, '--content-url', f'http://{{artifact}}.content.folio.localhost:{content_port}'],
                env={**os.environ, 'FOLIO_BOOTSTRAP_TOKEN': 'collaboration-fixture-setup'}, stdout=log, stderr=log)
            try:
                with sync_playwright() as p:
                    browser = p.chromium.launch(headless=True)
                    context = browser.new_context(viewport={'width': 1440, 'height': 1000}, accept_downloads=True)
                    page = context.new_page()
                    page.on('pageerror', lambda e: errors.append(str(e)))
                    page.on('console', console_failure)
                    for _ in range(100):
                        if proc.poll() is not None:
                            log.seek(0)
                            raise RuntimeError(log.read())
                        try:
                            if context.request.get(app + '/api/bootstrap', timeout=500).ok:
                                break
                        except Exception:
                            time.sleep(.1)
                    setup = context.request.post(app + '/api/setup', headers={'Origin': app}, data={
                        'token': 'collaboration-fixture-setup', 'organizationName': 'Collaboration fixture',
                        'name': 'Fixture Admin', 'email': 'collaboration@folio.test', 'password': 'Fixture-password-only-123!'})
                    assert setup.ok, setup.text()
                    csrf = setup.json()['csrfToken']
                    headers = {'Origin': app, 'X-CSRF-Token': csrf}

                    def request(method, path, body=None, want=200):
                        response = context.request.fetch(app + path, method=method, headers=headers, data=body)
                        assert response.status == want, (method, path, response.status, response.text())
                        return response.json() if response.status != 204 else None

                    def upload(title, number, aid='', base='', draft=False):
                        html = f'<!doctype html>\n<html lang="en"><meta charset="utf-8"><title>{title}</title><body><h1 id="marker">Version {number}</h1><p>Authored browser acceptance fixture.</p><script type="module" src="/app.js"></script><script type="module">document.body.dataset.asset=await(await fetch("asset.txt")).text()</script></body></html>'
                        response = context.request.post(app + '/api/artifacts' + (f'/{aid}/versions' if aid else ''), headers=headers, multipart={
                            'metadata': json.dumps({'title': title, 'description': 'Private draft description' if draft else 'Published description', 'baseVersionId': base, 'draft': draft, 'files': [{'path': 'index.html'}, {'path': 'asset.txt'}, {'path': 'app.js'}]}),
                            'file0': {'name': 'index.html', 'mimeType': 'text/html', 'buffer': html.encode()},
                            'file1': {'name': 'asset.txt', 'mimeType': 'text/plain', 'buffer': f'asset-{number}'.encode()},
                            'file2': {'name': 'app.js', 'mimeType': 'text/javascript', 'buffer': f'document.body.dataset.script="script-{number}";setTimeout(async()=>{{document.body.dataset.rootAsset=await(await fetch("/asset.txt")).text()}},100)'.encode()}})
                        assert response.status == 201, response.text()
                        return response.json()

                    first = upload('Collaboration report', 1)
                    aid, v1 = first['artifact']['id'], first['version']['id']
                    second = upload('Collaboration report', 2, aid, v1)
                    v2 = second['version']['id']
                    third = upload('Collaboration report draft', 3, aid, v2, True)
                    v3 = third['version']['id']
                    other = upload('Reference notes', 1)
                    other_id = other['artifact']['id']
                    # A public read response must retain the approved publication snapshot.
                    request('PATCH', f'/api/artifacts/{aid}/sharing', {'visibility': 'organization'})
                    member = request('POST', '/api/admin/members', {'name': 'Fixture reader', 'email': 'reader@folio.test', 'role': 'member', 'password': 'Reader-temporary-only-123!'}, want=201)
                    reader_context = browser.new_context()
                    login = reader_context.request.post(app + '/api/auth/login', headers={'Origin': app}, data={'email': 'reader@folio.test', 'password': 'Reader-temporary-only-123!'})
                    reader_csrf = login.json()['csrfToken']
                    password_change = reader_context.request.post(app + '/api/auth/password', headers={'Origin': app, 'X-CSRF-Token': reader_csrf}, data={'currentPassword': 'Reader-temporary-only-123!', 'newPassword': 'Reader-permanent-only-123!'})
                    assert password_change.ok
                    before_promotion = reader_context.request.get(app + f'/api/artifacts/{aid}')
                    assert before_promotion.ok
                    public_snapshot = before_promotion.json()
                    assert public_snapshot['artifact']['currentVersionId'] == v2
                    assert public_snapshot['artifact']['title'] == 'Collaboration report'
                    assert public_snapshot['artifact']['description'] == 'Published description'
                    assert 'Private draft description' not in before_promotion.text()
                    assert all(v['id'] != v3 for v in public_snapshot['versions'])
                    assert reader_context.request.get(app + f'/api/artifacts/{aid}/comments?versionId={v3}').status == 404
                    checks += ['draft-version-metadata-and-comments-hidden']
                    page.goto(app)
                    expect(page.get_by_role('heading', name='전체 아티팩트', exact=True)).to_be_visible()

                    def close_detail():
                        dialogs = page.get_by_role('dialog')
                        if dialogs.count():
                            dialogs.last.get_by_role('button', name='닫기', exact=True).click()
                            expect(page.get_by_role('dialog')).to_have_count(0)

                    def nav(name):
                        close_detail()
                        if page.viewport_size['width'] < 900:
                            # Native mobile navigation keeps the same named destinations.
                            toggler = page.get_by_role('button', name='메뉴 열기', exact=True)
                            if not toggler.count():
                                toggler = page.get_by_role('button', name='메뉴', exact=True)
                            toggler.click()
                        page.get_by_role('button', name=name, exact=True).first.click()

                    def open_report(tab='검토 및 피드백'):
                        nav('전체 아티팩트')
                        page.get_by_role('button', name='Collaboration report draft 열기', exact=True).click()
                        detail = page.get_by_role('dialog')
                        choose(detail.get_by_label('검토할 버전', exact=True), v3)
                        detail.get_by_role('button', name=tab, exact=True).click()
                        return detail

                    phase = 'review'
                    detail = open_report()
                    panel = detail.locator('.review-panel')
                    expect(panel.locator('.review-state')).to_be_visible()
                    panel.get_by_label('새 댓글', exact=True).fill('Check the heading before publication.')
                    panel.get_by_text('파일 위치 지정 (선택)', exact=True).click()
                    choose(panel.get_by_label('파일', exact=True), 'index.html')
                    panel.get_by_label('위치 또는 선택한 문구', exact=True).fill('Version 3')
                    panel.get_by_role('button', name='댓글 남기기', exact=True).click()
                    expect(panel.get_by_text('댓글을 남겼습니다.', exact=True)).to_be_visible()
                    thread = panel.locator('.review-thread').first
                    thread.get_by_role('button', name='답글', exact=True).click()
                    thread.get_by_label('답글', exact=True).fill('Verified against the source.')
                    thread.get_by_role('button', name='답글 남기기', exact=True).click()
                    expect(thread.get_by_text('Verified against the source.', exact=True)).to_be_visible()
                    thread.get_by_role('button', name='해결', exact=True).click()
                    expect(thread.get_by_text('해결됨', exact=True)).to_be_visible()
                    thread.get_by_role('button', name='다시 열기', exact=True).click()
                    choose(panel.get_by_label('검토 상태', exact=True), 'approved')
                    panel.get_by_role('button', name='상태 저장', exact=True).click()
                    expect(panel.locator('.review-state-approved')).to_be_visible()
                    panel.get_by_role('button', name='이 버전 게시', exact=True).click()
                    expect(panel.get_by_text('현재 게시된 버전입니다.', exact=True)).to_be_visible()
                    state = request('GET', f'/api/artifacts/{aid}')
                    assert state['artifact']['publishedVersionId'] == v3
                    checks += ['version-comments-replies-location', 'resolve-reopen', 'exact-review-approval-promotion']

                    # Collections exercise the native ordered controls rather than API reordering.
                    phase = 'collections'
                    nav('컬렉션')
                    page.get_by_role('button', name='새 컬렉션', exact=True).first.click()
                    editor = page.locator('.collection-editor')
                    editor.get_by_label('이름', exact=True).fill('Launch review')
                    editor.get_by_label('설명', exact=True).fill('Authored collaboration acceptance fixture.')
                    choose(editor.get_by_label('공유 범위', exact=True), 'organization')
                    editor.locator('.collection-choices').get_by_role('button').filter(has_text='Collaboration report').click()
                    editor.locator('.collection-choices').get_by_role('button').filter(has_text='Reference notes').click()
                    editor.get_by_role('button', name='Reference notes 위로 이동', exact=True).click()
                    editor.get_by_role('button', name='컬렉션 저장', exact=True).click()
                    expect(page.get_by_role('heading', name='Launch review', exact=True)).to_be_visible()
                    expect(page.locator('.collection-artifact-open').first).to_contain_text('Reference notes')
                    page.locator('.collection-artifact-open').filter(has_text='Collaboration report').click()
                    expect(page.get_by_role('dialog')).to_be_visible()
                    checks += ['collection-create-team-order-open', 'collection-permission-note']

                    phase = 'context'
                    detail = open_report('출처')
                    ctx = detail.locator('.artifact-context')
                    ctx.get_by_role('button', name='즐겨찾기', exact=True).click()
                    expect(ctx.get_by_role('button', name='즐겨찾기 해제', exact=True)).to_have_attribute('aria-pressed', 'true')
                    ctx.get_by_role('button', name='편집', exact=True).click()
                    ctx.get_by_label('태그', exact=True).fill('launch, reviewed')
                    ctx.get_by_label('에이전트 이름', exact=True).fill('Fixture agent')
                    ctx.get_by_label('모델', exact=True).fill('Authored test model')
                    ctx.get_by_label('요청 요약', exact=True).fill('Prepare the release report.')
                    ctx.get_by_label('저장소 URL', exact=True).fill('https://example.com/repository')
                    ctx.get_by_label('커밋', exact=True).fill('fixture-commit')
                    ctx.get_by_label('작업 URL', exact=True).fill('https://example.com/task')
                    ctx.get_by_label('출처 URL', exact=True).fill('https://example.com/source')
                    ctx.get_by_role('button', name='제작 정보 저장', exact=True).click()
                    expect(ctx.get_by_text('Fixture agent', exact=True)).to_be_visible()
                    ctx.get_by_text('버전 3의 제작 정보', exact=True).click()
                    expect(ctx.locator('.context-snapshot').get_by_text('등록된 제작 정보가 없습니다.', exact=True)).to_be_visible()
                    checks += ['favorite-tags-explicit-provenance', 'immutable-version-provenance-snapshot']

                    phase = 'comparison'
                    detail.get_by_role('button', name='버전 비교', exact=True).click()
                    comparison = detail.locator('.version-compare')
                    choose(comparison.get_by_label('이전 버전', exact=True), v1)
                    choose(comparison.get_by_label('이후 버전', exact=True), v3)
                    expect(comparison.locator('.compare-files button').filter(has_text='index.html')).to_be_visible()
                    comparison.locator('.compare-files button').filter(has_text='index.html').click()
                    expect(comparison.locator('.compare-code')).to_contain_text('Version 1')
                    expect(comparison.locator('.compare-code')).to_contain_text('Version 3')
                    frames = comparison.locator('iframe')
                    expect(frames).to_have_count(2)
                    for index, number in [(0, 1), (1, 3)]:
                        frame = frames.nth(index).element_handle().content_frame()
                        expect(frame.locator('#marker')).to_have_text(f'Version {number}')
                        expect(frame.locator('body')).to_have_attribute('data-asset', f'asset-{number}')
                        expect(frame.locator('body')).to_have_attribute('data-script', f'script-{number}')
                        expect(frame.locator('body')).to_have_attribute('data-root-asset', f'asset-{number}')
                    # Different versions have separate execution origins, including root-absolute assets.
                    origins = [frames.nth(i).element_handle().content_frame().evaluate("location.origin") for i in range(2)]
                    assert len(set(origins)) == 2 and app not in origins
                    for index, number in [(0, 1), (1, 3)]:
                        frame = frames.nth(index).element_handle().content_frame()
                        assert frame.evaluate('async()=>await(await fetch("asset.txt")).text()') == f'asset-{number}'
                        assert frame.evaluate('async()=>await(await fetch("/asset.txt")).text()') == f'asset-{number}'
                        assert f'script-{number}' in frame.evaluate('async()=>await(await fetch("/app.js")).text()')
                    checks += ['real-text-diff-sha-comparison', 'private-side-by-side-version-assets', 'isolated-version-root-absolute-modules-delayed-fetch']

                    close_detail()
                    page.get_by_role('button', name='Collaboration report draft 공유', exact=True).click()
                    share = page.get_by_role('dialog')
                    guest_section = share.locator('.ops-section')
                    choose(guest_section.get_by_label('공유할 버전', exact=True), v2)
                    guest_section.get_by_label('만료일', exact=True).fill((datetime.datetime.now() + datetime.timedelta(hours=2)).strftime('%Y-%m-%dT%H:%M'))
                    guest_section.get_by_label('비밀번호 (선택)', exact=True).fill('Guest-fixture-password-123!')
                    with page.expect_response(lambda r: r.request.method == 'POST' and r.url.endswith(f'/api/artifacts/{aid}/guest-links')) as response:
                        guest_section.get_by_role('button', name='게스트 링크 만들기', exact=True).click()
                    guest_link = response.value.json()['guestLink']
                    guest_context = browser.new_context(viewport={'width': 1440, 'height': 1000})
                    guest_page = guest_context.new_page()
                    guest_page.on('pageerror', lambda e: errors.append(str(e)))
                    guest_page.on('console', console_failure)
                    phase = 'guest-unlock'
                    guest_page.goto(guest_link['url'])
                    expect(guest_page.get_by_role('heading', name='공유된 아티팩트', exact=True)).to_be_visible()
                    guest_page.get_by_label('비밀번호', exact=True).fill('Guest-fixture-password-123!')
                    guest_page.get_by_role('button', name='아티팩트 열기', exact=True).click()
                    expect(guest_page.frame_locator('iframe').locator('#marker')).to_have_text('Version 2')
                    guest_frame = guest_page.locator('iframe').element_handle().content_frame()
                    expect(guest_frame.locator('body')).to_have_attribute('data-asset', 'asset-2')
                    expect(guest_frame.locator('body')).to_have_attribute('data-script', 'script-2')
                    expect(guest_frame.locator('body')).to_have_attribute('data-root-asset', 'asset-2')
                    content_url = guest_frame.url
                    phase = 'guest-revocation'
                    guest_section.get_by_role('button', name='링크 해제', exact=True).click()
                    expect(guest_section.get_by_text('해제됨', exact=True)).to_be_visible()
                    assert guest_context.request.get(content_url).status == 404
                    assert guest_context.request.get(app + '/api/me').status == 401
                    checks += ['password-guest-unlock-exact-version', 'guest-revocation-live-content-denied', 'guest-no-workspace-access']
                    close_detail()

                    # UI backup is downloaded before a destructive fixture-only change.
                    phase = 'backup-create'
                    nav('백업 및 복원')
                    backup = page.locator('.ops-section')
                    backup.locator('form').first.get_by_label('백업 비밀번호', exact=True).fill('Backup-fixture-password-123!')
                    backup.get_by_role('button', name='암호화 백업 만들기', exact=True).click()
                    expect(backup.get_by_text('암호화된 백업을 만들었습니다. 아래에서 다운로드하세요.', exact=True)).to_be_visible()
                    backup_file = Path(data) / 'snapshot.folio'
                    with page.expect_download() as download:
                        backup.get_by_role('link', name='다운로드', exact=True).first.click()
                    download.value.save_as(backup_file)
                    assert backup_file.stat().st_size > 100
                    phase = 'trash-restore'
                    request('DELETE', f'/api/artifacts/{other_id}', want=204)
                    nav('휴지통')
                    row = page.locator('.ops-list li').filter(has_text='Reference notes')
                    expect(row).to_be_visible()
                    row.get_by_role('button', name='복원', exact=True).click()
                    expect(page.get_by_text('휴지통이 비어 있습니다', exact=True)).to_be_visible()
                    request('DELETE', f'/api/artifacts/{other_id}', want=204)
                    nav('백업 및 복원')
                    phase = 'backup-restore'
                    restore = page.locator('.ops-restore')
                    restore.get_by_label('백업 파일', exact=True).set_input_files(backup_file)
                    restore.get_by_label('백업 비밀번호', exact=True).fill('Backup-fixture-password-123!')
                    restore.get_by_role('checkbox').check()
                    restore.get_by_role('button', name='백업 복원', exact=True).click()
                    expect(page.get_by_text('백업을 복원했습니다. 새로고침하면 복원된 아티팩트를 볼 수 있습니다.', exact=True)).to_be_visible()
                    restored = request('GET', f'/api/artifacts/{other_id}')
                    assert restored['artifact']['title'] == 'Reference notes'
                    assert request('GET', f'/api/artifacts/{aid}')['artifact']['publishedVersionId'] == v3
                    restored_comments = request('GET', f'/api/artifacts/{aid}/comments?versionId={v3}')['comments']
                    assert any(c['body'] == 'Check the heading before publication.' for c in restored_comments)
                    restored_collections = request('GET', '/api/collections')['collections']
                    assert next(c for c in restored_collections if c['name'] == 'Launch review')['artifactIds'] == [other_id, aid]
                    launch = request('POST', f'/api/artifacts/{aid}/launch', {'versionId': v1})
                    restored_content = context.request.get(launch['url'])
                    assert restored_content.ok and 'Version 1' in restored_content.text()
                    asset_url = restored_content.url.rsplit('/', 1)[0] + '/asset.txt'
                    assert context.request.get(asset_url).text() == 'asset-1'
                    checks += ['trash-ui-restore', 'encrypted-backup-ui-download', 'actual-backup-ui-restore-files-and-metadata']

                    # One batched desktop/mobile inspection after all functional flows.
                    def capture(name):
                        page.wait_for_timeout(150)
                        page.screenshot(path=str(shots / name), full_page=True)
                        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1'), name
                        dialogs = page.get_by_role('dialog')
                        if dialogs.count():
                            assert dialogs.last.evaluate('(e)=>e.scrollWidth <= e.clientWidth + 1'), name
                    phase = 'captures'
                    for width, suffix in [(1440, 'desktop'), (390, 'mobile')]:
                        page.set_viewport_size({'width': width, 'height': 1000 if width == 1440 else 844})
                        nav('컬렉션')
                        expect(page.get_by_role('heading', name='Launch review', exact=True)).to_be_visible()
                        capture(f'collections-{suffix}.png')
                        detail = open_report()
                        expect(detail.locator('.review-thread')).to_have_count(1)
                        capture(f'review-{suffix}.png')
                        detail.get_by_role('button', name='출처', exact=True).click()
                        expect(detail.get_by_text('Fixture agent', exact=True)).to_be_visible()
                        capture(f'context-{suffix}.png')
                        detail.get_by_role('button', name='버전 비교', exact=True).click()
                        expect(detail.locator('.compare-files button')).to_have_count(3)
                        capture(f'compare-{suffix}.png')
                        detail.locator('.compare-files-layout').scroll_into_view_if_needed()
                        capture(f'compare-diff-{suffix}.png')
                        nav('백업 및 복원')
                        expect(page.get_by_role('heading', name='백업과 복원', exact=True)).to_be_visible()
                        capture(f'backup-{suffix}.png')
                        nav('휴지통')
                        expect(page.get_by_role('heading', name='휴지통', exact=True)).to_be_visible()
                        capture(f'trash-{suffix}.png')
                    assert not errors, errors
                    checks += ['desktop-mobile-no-overflow', 'no-browser-page-errors']
                    browser.close()
                    result = {'status': 'PASS', 'checks': checks, 'pageErrors': errors, 'screenshots': str(shots)}
                    (shots / 'result.json').write_text(json.dumps(result, indent=2))
                    print(json.dumps(result))
            finally:
                proc.terminate()
                try:
                    proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()


if __name__ == '__main__':
    main()
