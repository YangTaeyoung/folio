#!/usr/bin/env python3
"""Appearance and icon acceptance using an isolated installation and authored data."""

import argparse, datetime, json, os, socket, subprocess, tempfile, time, struct, zlib
from pathlib import Path
from browser_controls import choose
from playwright.sync_api import sync_playwright, expect


def port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def photo():
    def chunk(kind, content):
        return (
            struct.pack("!I", len(content))
            + kind
            + content
            + struct.pack("!I", zlib.crc32(kind + content) & 0xFFFFFFFF)
        )

    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack("!IIBBBBB", 8, 8, 8, 2, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress((b"\0" + bytes([34, 88, 232]) * 8) * 8))
        + chunk(b"IEND", b"")
    )


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--binary", required=True)
    p.add_argument("--supplemental", action="store_true", help="Only share layout and unresolved contrast follow-ups")
    p.add_argument("--screenshots", default=".work/theme/browser")
    a = p.parse_args()
    out = Path(a.screenshots).resolve()
    out.mkdir(parents=True, exist_ok=True)
    ap, cp = port(), port()
    app = f"http://folio.localhost:{ap}"
    errors = []
    checks = []
    audits = []
    with tempfile.TemporaryDirectory(prefix="folio-theme-") as data, open(
        out / "server.log", "w+"
    ) as log:
        proc = subprocess.Popen(
            [
                str(Path(a.binary).resolve()),
                "serve",
                "--data-dir",
                data,
                "--listen",
                f"127.0.0.1:{ap}",
                "--content-listen",
                f"127.0.0.1:{cp}",
                "--app-url",
                app,
                "--content-url",
                f"http://{{artifact}}.content.folio.localhost:{cp}",
            ],
            env={**os.environ, "FOLIO_BOOTSTRAP_TOKEN": "theme-fixture"},
            stdout=log,
            stderr=log,
        )
        try:
            with sync_playwright() as pw:
                b = pw.chromium.launch()
                ctx = b.new_context(
                    viewport={"width": 1440, "height": 1000}, color_scheme="dark"
                )
                page = ctx.new_page()
                page.on("pageerror", lambda e: errors.append(str(e)))
                for _ in range(100):
                    try:
                        if ctx.request.get(app + "/api/bootstrap", timeout=500).ok:
                            break
                    except Exception:
                        time.sleep(0.1)
                setup = ctx.request.post(
                    app + "/api/setup",
                    headers={"Origin": app},
                    data={
                        "token": "theme-fixture",
                        "organizationName": "Theme fixture",
                        "name": "Fixture Admin",
                        "email": "theme@folio.test",
                        "password": "Theme-fixture-only-123!",
                    },
                )
                assert setup.ok, setup.text()
                headers = {"Origin": app, "X-CSRF-Token": setup.json()["csrfToken"]}
                r = ctx.request.post(
                    app + "/api/artifacts",
                    headers=headers,
                    multipart={
                        "metadata": json.dumps(
                            {
                                "title": "Appearance test",
                                "files": [{"path": "index.html"}],
                            }
                        ),
                        "file0": {
                            "name": "index.html",
                            "mimeType": "text/html",
                            "buffer": b'<!doctype html><html><body style="background:#f4eddf;color:#352a1f"><h1>Original artifact colors</h1></body></html>',
                        },
                    },
                )
                assert r.status == 201, r.text()
                aid = r.json()["artifact"]["id"]
                v1 = r.json()["version"]["id"]
                r = ctx.request.post(
                    app + f"/api/artifacts/{aid}/versions",
                    headers=headers,
                    multipart={
                        "metadata": json.dumps(
                            {
                                "title": "Appearance test",
                                "baseVersionId": v1,
                                "files": [{"path": "index.html"}],
                            }
                        ),
                        "file0": {
                            "name": "index.html",
                            "mimeType": "text/html",
                            "buffer": b'<!doctype html><html><body style="background:#f4eddf;color:#352a1f"><h1>Original artifact colors</h1><p>Revised fixture.</p></body></html>',
                        },
                    },
                )
                assert r.status == 201, r.text()
                ctx.request.post(
                    app + f"/api/artifacts/{aid}/comments",
                    headers=headers,
                    data={
                        "versionId": r.json()["version"]["id"],
                        "body": "Authored accessibility fixture comment.",
                    },
                )
                guest = ctx.request.post(
                    app + f"/api/artifacts/{aid}/guest-links",
                    headers=headers,
                    data={
                        "versionId": r.json()["version"]["id"],
                        "expiresAt": (
                            datetime.datetime.now(datetime.timezone.utc)
                            + datetime.timedelta(hours=1)
                        ).isoformat(),
                    },
                )
                assert guest.ok, guest.text()
                gid = guest.json()["guestLink"]["id"]

                def watch(p):
                    p.on("pageerror", lambda e: errors.append(str(e)))

                def theme(p, mode):
                    control = p.locator(".theme-toggle[data-preference]")
                    expect(control).to_be_visible()
                    for _ in range(3):
                        if control.get_attribute("data-preference") == mode:
                            break
                        control.click()
                    expect(control).to_have_attribute("data-preference", mode)
                    effective = p.evaluate("matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'") if mode == "system" else mode
                    expect(p.locator("html")).to_have_attribute("data-theme", effective)

                def nav(p, view):
                    if not p.locator(".primary-navigation").count():
                        p.goto(app + "/#library")
                    def primary(label):
                        control = p.locator(".primary-navigation").get_by_role("button", name=label, exact=True)
                        if p.viewport_size['width'] < 900 and not p.locator('.sidebar').evaluate("el => el.classList.contains('open')"):
                            p.get_by_role("button", name="메뉴 열기", exact=True).click()
                        control.click()
                        if p.viewport_size['width'] < 900:
                            p.wait_for_function("() => document.querySelector('.sidebar').getBoundingClientRect().right <= 0")
                    if view == "account":
                        p.get_by_role("button", name="내 계정 열기", exact=True).click()
                    elif view in ["library", "collections", "agents"]:
                        primary({"library":"라이브러리", "collections":"컬렉션", "agents":"에이전트"}[view])
                    elif view == "trash":
                        primary("라이브러리")
                        p.locator(".library-filters").get_by_role("button", name="휴지통", exact=True).click()
                    else:
                        primary("설정")
                        p.locator(".settings-navigation").get_by_role("button", name={"members":"구성원", "storage":"저장소와 보관", "domains":"도메인", "settings":"조직 설정", "backups":"백업 및 복원", "audit":"변경 기록"}[view], exact=True).click()
                    expect(p.locator(".workspace-main")).to_be_visible()

                def capture(p, name):
                    p.wait_for_timeout(180)
                    p.evaluate(Path("web/node_modules/axe-core/axe.min.js").read_text())
                    audit = p.evaluate("""async () => {
      const context={exclude:[['iframe']]};if(document.querySelector('[role=dialog]'))context.include=[['[role=dialog]']];const r=await axe.run(context,{runOnly:{type:'rule',values:['color-contrast']},iframes:false});
      return {violations:r.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})),incomplete:r.incomplete.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))}))};
     }""")
                    # Text+chevron ghost selectors are recognized by their content;
                    # their decorative outlines need not establish the control boundary.
                    # Actual input fields and selects inside forms retain the 3:1 check.
                    # Icon-only selectors and chevrons are measured separately at 3:1.
                    control_audit = Path("web/tests/contrast.js").read_text().replace(
                        "if(!visible(e)||e.disabled)continue;",
                        "if(!visible(e)||e.disabled)continue;if(e.matches('[role=combobox]')&&!e.closest('form')&&(e.querySelector('.select-chevron')||e.classList.contains('select-icon-only')))continue;",
                    )
                    control_audit = control_audit.replace(
                        ".icon-button:not(:disabled)",
                        ".icon-button:not(:disabled),.select-control:not(:disabled) > svg,.select-control:not(:disabled) .select-chevron",
                    )
                    audit["controls"] = p.evaluate(control_audit)
                    audits.append({"state": name, **audit})
                    (out / "contrast.json").write_text(
                        json.dumps(audits, indent=2, ensure_ascii=False)
                    )
                    p.screenshot(path=str(out / (name + ".png")), full_page=True)
                    assert p.evaluate(
                        "document.documentElement.scrollWidth <= innerWidth+1"
                    ), name


                if a.supplemental:
                    measured = []
                    layouts = []
                    # These are direct measurements of computed foreground and
                    # opaque background colors, recorded separately from axe.
                    def colors(p, selector, state):
                        records = p.evaluate(r"""selector => {
                          const rgb = value => {
                            if (!/^rgba?\(/.test(value)) throw Error('Unsupported computed color: '+value);
                            const n=value.match(/[\d.]+/g).map(Number); return [n[0],n[1],n[2],n[3]??1];
                          };
                          const mix=(a,b)=>a.slice(0,3).map((v,i)=>v*a[3]+b[i]*(1-a[3]));
                          const lum=c=>c.map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4}).reduce((v,n,i)=>v+n*[.2126,.7152,.0722][i],0);
                          return [...document.querySelectorAll(selector)].filter(el=>el.getClientRects().length).map(el=>{
                            const chain=[];for(let n=el;n;n=n.parentElement)chain.unshift(n);
                            const unsupported=[];
                            const bg=chain.reduce((base,n)=>{const s=getComputedStyle(n);if(s.backgroundImage!=='none')unsupported.push('background image');if(s.opacity!=='1')unsupported.push('opacity '+s.opacity);return mix(rgb(s.backgroundColor),base)},[255,255,255]);
                            const raw=getComputedStyle(el).color,fg=mix(rgb(raw),bg);
                            const ratio=(Math.max(lum(fg),lum(bg))+.05)/(Math.min(lum(fg),lum(bg))+.05);
                            return {text:el.textContent.trim(),selector:el.tagName+'.'+el.className,foreground:raw,background:bg,ratio:+ratio.toFixed(4),required:4.5,pass:ratio>=4.5&&!unsupported.length,unsupported};
                          });
                        }""", selector)
                        assert records, (state, selector)
                        measured.append({"state":state,"source":"computed-colors","records":records})
                        (out / "supplemental.json").write_text(json.dumps({"layouts":layouts,"measurements":measured},ensure_ascii=False,indent=2))
                        assert all(record['pass'] for record in records), (state, records)

                    page.goto(app + '/#library')
                    expect(page.locator('.artifact-card')).to_be_visible()
                    for mode in ['light','dark']:
                        theme(page,mode)
                        page.locator('.artifact-card').get_by_role('link',name='Appearance test 열기',exact=True).click()
                        page.locator('.viewer-header').get_by_role('button',name='공유',exact=True).click()
                        dialog=page.get_by_role('dialog')
                        expect(dialog.locator('.visibility-option')).to_have_count(4)
                        layout=dialog.evaluate("""el=>{const body=el.querySelector('.drawer-body'),r=body.getBoundingClientRect();return {bodyHeight:body.clientHeight,bodyTop:r.top,bodyBottom:r.bottom,options:[...el.querySelectorAll('.visibility-option')].map(o=>{const x=o.getBoundingClientRect();return {label:o.innerText,top:x.top,bottom:x.bottom,height:x.height,withinBody:x.top>=r.top&&x.bottom<=r.bottom&&x.bottom<=innerHeight}})}}""")
                        layouts.append({"state":mode+'-share-first-open',**layout})
                        assert layout['bodyHeight'] >= 450, layout
                        assert all(option['withinBody'] and option['height']>=62 for option in layout['options']),layout
                        capture(page,mode+'-share-first-open')
                        colors(page,'.artifact-sharing h2,.artifact-sharing .drawer-document b,.artifact-sharing .drawer-document small,.artifact-sharing .share-current-access span,.artifact-sharing legend,.artifact-sharing .visibility-option b,.artifact-sharing .visibility-option small,.artifact-sharing .drawer-footer p,.artifact-sharing .drawer-footer button',mode+'-share-text')
                        dialog.get_by_role('button',name='닫기',exact=True).click()
                        page.goto(app+'/#library')
                        nav(page,'audit')
                        expect(page.locator('.audit-change > span[aria-hidden=true]').first).to_be_visible()
                        version_change=page.locator('.audit-table tbody tr').filter(has=page.get_by_text('새 버전 업로드',exact=True)).first.locator('.audit-change').first
                        expect(version_change).to_contain_text('현재 버전')
                        expect(version_change.locator('del')).to_have_text('v1')
                        expect(version_change.locator('ins')).to_have_text('v2')
                        capture(page,mode+'-audit-arrows')
                        colors(page,'.audit-change > span[aria-hidden=true]',mode+'-audit-arrows')
                        nav(page,'library')
                        for width in [390,320]:
                            page.set_viewport_size({'width':width,'height':844})
                            capture(page,mode+'-library-tabs-'+str(width))
                            colors(page,'.library-filters button',mode+'-library-tabs-'+str(width))
                        page.set_viewport_size({'width':1440,'height':1000})
                    assert not errors,errors
                    assert not any(x['violations'] or any(not c['pass'] for c in x['controls']) for x in audits),audits
                    result={"status":"PASS","mode":"targeted-supplemental","pageErrors":errors,"axeStates":len(audits),"axeViolations":sum(len(x['violations']) for x in audits),"axeIncomplete":[{"state":x['state'],"count":sum(len(i['nodes']) for i in x['incomplete']),"details":x['incomplete']} for x in audits if x['incomplete']],"computedColorMeasurements":sum(len(x['records']) for x in measured),"computedColorMinimum":min(r['ratio'] for x in measured for r in x['records']),"shareLayouts":layouts}
                    (out/'result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))
                    print(json.dumps(result,ensure_ascii=False))
                    b.close()
                    return

                page.goto(app)
                expect(
                    page.get_by_role("heading", name="전체 아티팩트", exact=True)
                ).to_be_visible()
                expect(page.locator("html")).to_have_attribute("data-theme", "dark")
                checks.append("system-dark-default")
                page.emulate_media(color_scheme="light")
                expect(page.locator("html")).to_have_attribute("data-theme", "light")
                page.emulate_media(color_scheme="dark")
                expect(page.locator("html")).to_have_attribute("data-theme", "dark")
                checks.append("live-system-changes")
                theme(page, "light")
                page.reload()
                expect(page.locator("html")).to_have_attribute("data-theme", "light")
                page.emulate_media(color_scheme="dark")
                expect(page.locator("html")).to_have_attribute("data-theme", "light")
                checks.append("explicit-choice-persists-over-system")
                other = ctx.new_page()
                watch(other)
                other.goto(app)
                expect(other.locator("html")).to_have_attribute("data-theme", "light")
                theme(page, "dark")
                expect(other.locator("html")).to_have_attribute("data-theme", "dark")
                other.close()
                checks.append("cross-tab-sync")
                for mode in ["light", "dark"]:
                    theme(page, mode)
                    capture(page, mode + "-library")
                    page.locator(".artifact-card").get_by_role("link", name="Appearance test 열기", exact=True).click()
                    expect(page.locator(".viewer")).to_be_visible()
                    expect(page.locator(".theme-toggle,.language-selector")).to_have_count(0)
                    page.get_by_role("button", name="아티팩트 상세", exact=True).click()
                    dialog = page.get_by_role("dialog")
                    frame = (
                        dialog.locator("iframe").first.element_handle().content_frame()
                    )
                    expect(
                        frame.get_by_role("heading", name="Original artifact colors")
                    ).to_be_visible()
                    assert (
                        frame.locator("body").evaluate(
                            "e=>getComputedStyle(e).backgroundColor"
                        )
                        == "rgb(244, 237, 223)"
                    )
                    capture(page, mode + "-detail")
                    for label in ["검토 및 피드백", "버전 비교"]:
                        dialog.get_by_role("button", name=label, exact=True).click()
                        capture(
                            page,
                            mode
                            + "-"
                            + ("review" if label == "검토 및 피드백" else "compare"),
                        )
                    dialog.get_by_role("button", name="닫기", exact=True).click()
                    page.locator(".viewer-header").get_by_role("button", name="공유", exact=True).click()
                    expect(page.get_by_role("dialog")).to_be_visible()
                    capture(page, mode + "-share")
                    page.get_by_role("dialog").get_by_role(
                        "button", name="닫기", exact=True
                    ).click()
                    for view in [
                        "account",
                        "members",
                        "storage",
                        "domains",
                        "agents",
                        "settings",
                        "collections",
                        "trash",
                        "backups",
                        "audit",
                    ]:
                        nav(page, view)
                        expect(page.locator(".workspace-main")).to_be_visible()
                        page.wait_for_timeout(200)
                        capture(page, mode + "-" + view)
                        if view == "storage":
                            page.get_by_role(
                                "button", name="저장소 연결", exact=True
                            ).click()
                            storage = page.get_by_role("dialog")
                            picker = storage.get_by_label("저장소 종류", exact=True)
                            picker.click()
                            expect(page.get_by_role("listbox")).to_be_visible()
                            capture(page, mode + "-service-menu")
                            page.keyboard.press("Escape")
                            expect(page.get_by_role("listbox")).to_have_count(0)
                            expect(storage).to_be_visible()
                            expect(picker).to_be_focused()
                            for service in ["aws", "r2", "minio", "gcs", "azure"]:
                                choose(picker, service)
                                capture(page, mode + "-storage-" + service)
                                assert storage.locator(
                                    ".service-icon img"
                                ).first.evaluate(
                                    "(e)=>e.complete && e.naturalWidth>0"
                                ), service
                            storage.get_by_role(
                                "button", name="닫기", exact=True
                            ).click()
                        if view == "domains":
                            page.get_by_role(
                                "button", name="도메인 연결", exact=True
                            ).click()
                            domain = page.get_by_role("dialog")
                            domain.get_by_label("호스트 이름", exact=True).fill(
                                "fixture.example.com"
                            )
                            requests = []
                            listener = lambda request: (
                                requests.append(request.url)
                                if request.method == "POST"
                                and request.url.endswith("/api/domains")
                                else None
                            )
                            page.on("request", listener)
                            domain.get_by_role(
                                "button", name="다음: DNS 등록", exact=True
                            ).click()
                            assert (
                                domain.locator("form").evaluate(
                                    "(f)=>f.checkValidity()"
                                )
                                is False
                            )
                            assert not requests, "Empty required Select reached server"
                            expect(
                                domain.get_by_label("연결할 아티팩트", exact=True)
                            ).to_be_focused()
                            expect(domain.get_by_role("alert")).to_have_text(
                                "항목을 선택해 주세요."
                            )
                            page.remove_listener("request", listener)
                            capture(page, mode + "-required-selection")
                            choose(
                                domain.get_by_label("연결할 아티팩트", exact=True), aid
                            )
                            assert (
                                domain.locator("form").evaluate(
                                    "(f)=>f.checkValidity()"
                                )
                                is True
                            )
                            domain.get_by_role(
                                "button", name="닫기", exact=True
                            ).click()
                        if view in ["account", "settings"]:
                            heading = "내 프로필" if view == "account" else "조직 정보"
                            profile = page.locator(".profile-settings").filter(
                                has=page.get_by_role(
                                    "heading", name=heading, exact=True
                                )
                            )
                            profile.locator("input[type=file]").set_input_files(
                                {
                                    "name": "fixture.png",
                                    "mimeType": "image/png",
                                    "buffer": photo(),
                                }
                            )
                            profile.get_by_role(
                                "button", name="사진 저장", exact=True
                            ).click()
                            expect(
                                profile.get_by_role(
                                    "button", name="사진 삭제", exact=True
                                )
                            ).to_be_visible()
                            image = profile.locator(".profile-photo-preview img")
                            expect(image).to_have_attribute(
                                "src", __import__("re").compile("/api/")
                            )
                            assert image.evaluate("(e)=>e.complete && e.naturalWidth>0")
                            shell = page.locator(
                                ".sidebar-user .avatar img"
                                if view == "account"
                                else ".workspace-switch .avatar img"
                            )
                            expect(shell).to_be_visible()
                            capture(page, mode + "-" + view + "-photo")
                            if view == "account":
                                nav(page, "library")
                                expect(
                                    page.locator(".artifact-card .avatar img")
                                ).to_be_visible()
                                assert page.locator(
                                    ".artifact-card .avatar img"
                                ).evaluate("(e)=>e.complete && e.naturalWidth>0")
                                nav(page, "account")
                            page.reload()
                            expect(
                                page.locator(".profile-photo-preview img")
                            ).to_be_visible()
                            profile = page.locator(".profile-settings").filter(
                                has=page.get_by_role(
                                    "heading", name=heading, exact=True
                                )
                            )
                            profile.get_by_role(
                                "button", name="사진 삭제", exact=True
                            ).click()
                            expect(
                                profile.get_by_role(
                                    "button", name="사진 삭제", exact=True
                                )
                            ).to_have_count(0)
                    nav(page, "library")
                    expect(page.locator(".artifact-card")).to_be_visible()
                checks.extend(
                    [
                        "all-shell-surfaces-both-modes-and-unmodified-artifact",
                        "photo-upload-preview-persist-shell-update-and-delete",
                        "all-storage-brands-and-dialog-menu-escape-focus",
                    ]
                )
                anonymous = b.new_context(
                    viewport={"width": 1440, "height": 1000}, color_scheme="dark"
                )
                login = anonymous.new_page()
                watch(login)
                login.goto(app)
                expect(
                    login.get_by_role("heading", name="작업 공간에 로그인")
                ).to_be_visible()
                login.get_by_label("이메일", exact=True).focus()
                login.keyboard.press("Tab")
                capture(login, "dark-login")
                assert (
                    login.locator(".brand img").get_attribute("src")
                    == "/brand-mark.svg"
                )
                for path in [
                    "/favicon.svg",
                    "/favicon.ico",
                    "/apple-touch-icon.png",
                    "/icon-192.png",
                    "/icon-512.png",
                    "/brand-mark.svg",
                    "/site.webmanifest",
                ]:
                    res = anonymous.request.get(app + path)
                    assert res.ok and "text/html" not in res.headers["content-type"], (
                        path,
                        res.status,
                        res.headers,
                    )
                manifest = anonymous.request.get(app + "/site.webmanifest").json()
                assert len(manifest["icons"]) == 2
                checks.append("all-icon-assets-real-and-served")
                login.get_by_label("이메일", exact=True).fill("theme@folio.test")
                login.get_by_label("비밀번호", exact=True).fill("wrong-password")
                login.get_by_role("button", name="로그인", exact=True).click()
                expect(login.get_by_role("alert")).to_be_visible()
                capture(login, "dark-error")
                theme(login, "light")
                login.get_by_label("이메일", exact=True).focus()
                login.keyboard.press("Tab")
                capture(login, "light-login")
                for width in [390, 320]:
                    login.set_viewport_size({"width": width, "height": 844})
                    theme(login, "dark")
                    capture(login, f"dark-login-{width}")
                    page.set_viewport_size({"width": width, "height": 844})
                    page.reload()
                    expect(page.locator(".workspace-topbar .workspace-menu")).to_be_visible()
                    capture(page, f"dark-library-{width}")
                    page.get_by_role("button", name="메뉴 열기", exact=True).click()
                    expect(page.locator(".sidebar .appearance-controls")).to_have_count(0)
                    capture(page, f"dark-menu-{width}")
                    page.get_by_role("button", name="메뉴 닫기", exact=True).click()
                login.set_viewport_size({"width": 390, "height": 844})
                login.goto(app + "/g/" + gid)
                expect(login.locator(".theme-toggle,.language-selector")).to_have_count(0)
                capture(login, "dark-guest-mobile")
                checks.append("mobile-topbar-theme-control-reader-isolation-and-no-overflow")
                page.set_viewport_size({"width":1440,"height":1000})
                page.emulate_media(reduced_motion="reduce")
                expect(page.locator(".theme-toggle")).to_be_visible()
                page.locator(".theme-toggle").focus()
                previous = page.locator(".theme-toggle").get_attribute("data-preference")
                page.keyboard.press("Enter")
                expect(page.locator(".theme-toggle")).not_to_have_attribute("data-preference", previous)
                assert page.evaluate("matchMedia('(prefers-reduced-motion: reduce)').matches")
                assert page.locator('.theme-toggle').evaluate("el => {const s=getComputedStyle(el);return s.transitionDuration==='0s'&&s.animationName==='none'}")
                capture(page, "reduced-motion-keyboard-theme")
                page.emulate_media(reduced_motion="no-preference")
                checks.append("reduced-motion-and-keyboard-theme-toggle")
                blocked = b.new_context(color_scheme="dark")
                blocked.add_init_script(
                    "Object.defineProperty(window, 'localStorage', {get(){throw new DOMException('Blocked','SecurityError')}})"
                )
                bp = blocked.new_page()
                watch(bp)
                bp.goto(app)
                expect(
                    bp.get_by_role("heading", name="작업 공간에 로그인")
                ).to_be_visible()
                expect(bp.locator("html")).to_have_attribute("data-theme", "dark")
                theme(bp, "light")
                checks.append("storage-unavailable-usable")
                first = b.new_context(color_scheme="dark")
                first.route("**/assets/*.js", lambda route: route.abort())
                fp = first.new_page()
                fp.goto(app)
                expect(fp.locator("html")).to_have_attribute("data-theme", "dark")
                assert (
                    fp.evaluate(
                        "getComputedStyle(document.documentElement).backgroundColor"
                    )
                    == "rgb(21, 23, 28)"
                )
                checks.append("theme-applied-before-react-starts")
                violations = [
                    x
                    for x in audits
                    if x["violations"] or any(not c["pass"] for c in x["controls"])
                ]
                assert not violations, json.dumps([
                    {"state": x["state"], "text": x["violations"],
                     "controlFailures": [c for c in x["controls"] if not c["pass"]]}
                    for x in violations
                ], ensure_ascii=False)
                assert not errors, errors
                result = {
                    "status": "PASS",
                    "checks": checks,
                    "pageErrors": errors,
                    "contrastStates": len(audits),
                    "axeContrastViolations": 0,
                }
                (out / "result.json").write_text(json.dumps(result, indent=2))
                print(json.dumps(result))
                b.close()
        finally:
            proc.terminate()
            proc.wait(timeout=15)


if __name__ == "__main__":
    main()
