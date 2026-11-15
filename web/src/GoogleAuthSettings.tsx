import { useEffect, useState } from "react";
import { CheckCircle2, ExternalLink, RefreshCw } from "lucide-react";
import { api, json } from "./api";
import { CopyButton, ErrorBox, Field, Notice, Spinner, useAction } from "./ui";
import "./GoogleAuthSettings.css";

interface GoogleSettings {
  enabled: boolean;
  clientId: string;
  clientSecretConfigured: boolean;
  allowedDomains: string[];
  redirectURI: string;
}

export function GoogleAuthSettings() {
  const [config, setConfig] = useState<GoogleSettings | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [domains, setDomains] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [revision, setRevision] = useState(0);
  const [saved, setSaved] = useState(false);
  const a = useAction();

  function apply(value: GoogleSettings) {
    setConfig(value);
    setEnabled(value.enabled);
    setClientId(value.clientId);
    setDomains(value.allowedDomains.join(", "));
    setClientSecret("");
  }

  useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadError(null);
    api<GoogleSettings>("/api/admin/auth/google")
      .then((value) => {
        if (active) apply(value);
      })
      .catch((error) => {
        if (active) setLoadError(error);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [revision]);

  return (
    <section
      className="panel google-auth-settings"
      aria-labelledby="google-auth-heading"
    >
      <div className="section-title">
        <h2 id="google-auth-heading">Google 로그인</h2>
        {config && (
          <span className={`badge ${config.enabled ? "" : "muted"}`}>
            {config.enabled ? "사용 중" : "사용 안 함"}
          </span>
        )}
      </div>
      <p className="subtle">
        설치 관리자가 연결하는 선택 기능입니다. 기본값은 꺼짐이며, 비밀번호
        로그인은 Google 연결 없이 사용할 수 있습니다.
      </p>
      {loading ? (
        <div className="loading-row">
          <Spinner />
          Google 로그인 설정 불러오는 중
        </div>
      ) : loadError ? (
        <>
          <ErrorBox error={loadError} />
          <button
            type="button"
            className="button secondary"
            onClick={() => setRevision((value) => value + 1)}
          >
            <RefreshCw size={15} />
            다시 불러오기
          </button>
        </>
      ) : (
        config && (
          <>
            <form
              onSubmit={(event) => {
                event.preventDefault();
                setSaved(false);
                void a.run(async () => {
                  const value = await api<GoogleSettings>(
                    "/api/admin/auth/google",
                    json("PATCH", {
                      enabled,
                      clientId: clientId.trim(),
                      allowedDomains: domains
                        .split(",")
                        .map((domain) => domain.trim().toLowerCase())
                        .filter(Boolean),
                      ...(clientSecret.trim() ? { clientSecret } : {}),
                    }),
                  );
                  apply(value);
                  setSaved(true);
                });
              }}
              onChange={() => setSaved(false)}
            >
              <fieldset className="google-auth-fields" disabled={a.busy}>
                <label className="google-auth-enable">
                  <input
                    type="checkbox"
                    checked={enabled}
                    onChange={(event) => setEnabled(event.target.checked)}
                  />
                  Google 로그인 사용
                </label>
                <div className="form-grid">
                  <Field
                    label="Client ID"
                    hint="이 설치를 위해 만든 웹 애플리케이션 OAuth 클라이언트 ID입니다."
                  >
                    <input
                      required={enabled}
                      value={clientId}
                      autoComplete="off"
                      spellCheck={false}
                      placeholder="…apps.googleusercontent.com"
                      onChange={(event) => setClientId(event.target.value)}
                    />
                  </Field>
                  <Field
                    label="Client Secret"
                    hint={
                      config.clientSecretConfigured
                        ? "비밀 키가 저장되어 있습니다. 비워 두면 기존 값을 유지합니다."
                        : "Google Cloud에서 발급한 비밀 키를 입력합니다. 저장된 값은 다시 표시하지 않습니다."
                    }
                  >
                    <input
                      type="password"
                      required={enabled && !config.clientSecretConfigured}
                      autoComplete="new-password"
                      value={clientSecret}
                      placeholder={
                        config.clientSecretConfigured
                          ? "저장된 비밀 키 유지"
                          : "Client Secret 입력"
                      }
                      onChange={(event) => setClientSecret(event.target.value)}
                    />
                  </Field>
                </div>
                <Field
                  label="허용할 Google Workspace 도메인 (선택)"
                  hint="예: example.com, team.example.com. 입력하면 해당 Workspace 조직의 계정만 허용합니다. 비워 두면 Gmail과 Google Workspace 계정을 허용합니다. 어느 경우든 Folio에 미리 등록한 활성 계정이어야 합니다."
                >
                  <input
                    value={domains}
                    autoComplete="off"
                    spellCheck={false}
                    placeholder="example.com, team.example.com"
                    onChange={(event) => setDomains(event.target.value)}
                  />
                </Field>
                <ErrorBox error={a.error} />
                <div className="google-auth-actions">
                  <button className="button primary" disabled={a.busy}>
                    {a.busy ? (
                      <>
                        <Spinner />
                        저장 중
                      </>
                    ) : (
                      "Google 로그인 설정 저장"
                    )}
                  </button>
                  {saved && (
                    <span className="saved-inline" role="status">
                      <CheckCircle2 size={15} />
                      저장했습니다.
                    </span>
                  )}
                </div>
              </fieldset>
            </form>
            <details className="google-auth-guide admin-disclosure">
              <summary>
                <span className="admin-disclosure-title">
                  Google Cloud에서 연결하기
                </span>
                <span className="subtle">설정 순서와 콜백 주소</span>
              </summary>
              <div className="admin-disclosure-body">
                <ol>
                  <li>
                    이 설치 소유자의 Google Cloud 프로젝트에서 Google Auth
                    Platform의 브랜딩과 대상을 설정합니다. 테스트 상태라면
                    로그인할 계정을 테스트 사용자로 추가합니다.
                  </li>
                  <li>
                    클라이언트 유형을 <b>웹 애플리케이션</b>으로 만들고, 아래
                    주소를 <b>승인된 리디렉션 URI</b>에 그대로 등록합니다.
                  </li>
                  <li>
                    Client ID와 Client Secret을 위에 입력하고 Google 로그인을 켠
                    뒤 저장합니다.
                  </li>
                  <li>
                    구성원 메뉴에서 사용할 이메일을 미리 등록한 후 Google
                    로그인을 확인합니다. 기존 비밀번호 관리자 계정은 복구용으로
                    유지해 주세요.
                  </li>
                </ol>
                <Field
                  label="승인된 리디렉션 URI"
                  hint="현재 서버의 관리 주소로 생성한 콜백 주소입니다. Google Cloud에 동일하게 등록합니다."
                >
                  <input
                    readOnly
                    value={config.redirectURI}
                    spellCheck={false}
                    onFocus={(event) => event.target.select()}
                  />
                </Field>
                <div className="google-auth-links">
                  <CopyButton
                    text={config.redirectURI}
                    label="콜백 주소 복사"
                  />
                  <a
                    href="https://developers.google.com/identity/openid-connect/openid-connect"
                    target="_blank"
                    rel="noreferrer"
                  >
                    Google 로그인 공식 안내 <ExternalLink size={13} />
                  </a>
                  <a
                    href="https://developers.google.com/workspace/guides/create-credentials"
                    target="_blank"
                    rel="noreferrer"
                  >
                    OAuth 클라이언트 만들기 <ExternalLink size={13} />
                  </a>
                </div>
              </div>
            </details>
            <Notice>
              Google 로그인은 자동 가입이나 관리자 권한을 부여하지 않습니다.
              Gmail 또는 Google Workspace에서 확인한 이메일이 기존 계정과
              일치해야 합니다. 이 기능을 사용할 때만 Google에 연결합니다.
            </Notice>
          </>
        )
      )}
    </section>
  );
}
