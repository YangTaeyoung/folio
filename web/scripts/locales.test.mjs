import test from "node:test";
import assert from "node:assert/strict";
import { checkCatalogs, loadCatalogs } from "./check-locales.mjs";
test("all bundled locales cover source strings and preserve interpolation parameters", () => {
  assert.equal(checkCatalogs().locales, 5);
});
test("security and recovery instructions have distinct translations", () => {
  const catalogs = loadCatalogs();
  for (const key of [
    "링크를 복사해도 공유 범위는 바뀌지 않습니다.",
    "Google 로그인은 자동 가입이나 관리자 권한을 부여하지 않습니다. Gmail 또는 Google Workspace에서 확인한 이메일이 기존 계정과 일치해야 합니다. 이 기능을 사용할 때만 Google에 연결합니다.",
    "작성자와 관리자만 열 수 있습니다.",
  ]) {
    assert.equal(
      new Set(Object.values(catalogs).map((catalog) => catalog[key])).size,
      5,
    );
  }
});
