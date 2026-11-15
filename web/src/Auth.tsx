import { useState } from "react";
import { Eye, EyeOff, KeyRound } from "lucide-react";
import { api, json, setCSRF } from "./api";
import type { Session } from "./types";
import { ErrorBox, Field, Spinner, useAction } from "./ui";
export function Brand() {
  return (
    <span className="brand">
      <img src="/favicon.svg" alt="" />
      Folio
    </span>
  );
}
export default function Auth({
  setup,
  onLogin,
}: {
  setup: boolean;
  onLogin: (s: Session) => void;
}) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [name, setName] = useState("");
  const [org, setOrg] = useState("");
  const [token, setToken] = useState(
    () => new URLSearchParams(location.hash.slice(1)).get("setup") || "",
  );
  const action = useAction();
  return (
    <main className={`auth-page ${setup ? "auth-setup" : ""}`}>
      <header className="auth-header">
        <Brand />
      </header>
      <section className="auth-form-wrap">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void action.run(async () => {
              const s = await api<Session>(
                setup ? "/api/setup" : "/api/auth/login",
                json(
                  "POST",
                  setup
                    ? { email, password, name, organizationName: org, token }
                    : { email, password },
                ),
              );
              setCSRF(s.csrfToken);
              history.replaceState({}, "", location.pathname);
              onLogin(s);
            });
          }}
        >
          <img className="auth-mark" src="/favicon.svg" alt="" />
          <h2>{setup ? "작업 공간 만들기" : "작업 공간에 로그인"}</h2>
          <p className="subtle">
            {setup
              ? "조직과 첫 관리자 계정을 설정하세요."
              : "이메일과 비밀번호로 작업 공간에 접속하세요."}
          </p>
          {setup && (
            <>
              <Field
                label="설치 키"
                hint="서버 설치 시 표시된 일회용 설치 키를 입력하세요."
              >
                <input
                  required
                  type="password"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="조직 이름">
                <input
                  required
                  value={org}
                  onChange={(e) => setOrg(e.target.value)}
                  placeholder="예: Melona"
                />
              </Field>
              <Field label="이름">
                <input
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  autoComplete="name"
                />
              </Field>
            </>
          )}
          <Field label="이메일">
            <input
              required
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="username"
              placeholder="you@company.com"
            />
          </Field>
          <div className="field auth-password-field">
            <label htmlFor="auth-password">비밀번호</label>
            <div className="auth-password-input">
              <input
                id="auth-password"
                required
                type={showPassword ? "text" : "password"}
                minLength={setup ? 12 : undefined}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete={setup ? "new-password" : "current-password"}
                aria-describedby={setup ? "auth-password-hint" : undefined}
              />
              <button
                type="button"
                className="icon-button"
                aria-label={showPassword ? "비밀번호 숨기기" : "비밀번호 표시"}
                aria-pressed={showPassword}
                onClick={() => setShowPassword((value) => !value)}
              >
                {showPassword ? <EyeOff size={17} /> : <Eye size={17} />}
              </button>
            </div>
            {setup && (
              <small id="auth-password-hint">
                12자 이상으로 설정해 주세요.
              </small>
            )}
          </div>
          <ErrorBox error={action.error} />
          <button className="button primary auth-submit" disabled={action.busy}>
            {action.busy ? <Spinner /> : setup ? "작업 공간 만들기" : "로그인"}
          </button>
          {!setup && (
            <div className="auth-account-help">
              <p>
                처음 이용하시나요?
                <br />
                조직 관리자가 생성한 계정으로 로그인할 수 있습니다.
              </p>
              <details>
                <summary>계정 도움말</summary>
                <p>
                  계정이 없거나 비밀번호를 잊으셨다면 조직 관리자에게
                  문의하세요. 관리자가 계정을 생성하거나 비밀번호를 초기화할 수
                  있습니다.
                </p>
              </details>
            </div>
          )}
        </form>
      </section>
    </main>
  );
}
export function PasswordChange({
  onChanged,
}: {
  onChanged: (s: Partial<Session>) => void;
}) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const a = useAction();
  return (
    <div className="password-gate">
      <form
        className="panel narrow"
        onSubmit={(e) => {
          e.preventDefault();
          void a.run(async () => {
            if (next !== confirm)
              throw new Error("새 비밀번호가 일치하지 않습니다.");
            const s = await api<Partial<Session>>(
              "/api/auth/password",
              json("POST", { currentPassword: current, newPassword: next }),
            );
            if (s.csrfToken) setCSRF(s.csrfToken);
            onChanged(s);
          });
        }}
      >
        <KeyRound size={28} />
        <h1>나만의 비밀번호로 바꿔주세요</h1>
        <p className="subtle">처음 로그인할 때 임시 비밀번호를 변경합니다.</p>
        <Field label="임시 비밀번호">
          <input
            required
            type="password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            autoComplete="current-password"
          />
        </Field>
        <Field label="새 비밀번호 · 12자 이상">
          <input
            required
            type="password"
            minLength={12}
            value={next}
            onChange={(e) => setNext(e.target.value)}
            autoComplete="new-password"
          />
        </Field>
        <Field label="새 비밀번호 확인">
          <input
            required
            type="password"
            minLength={12}
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
          />
        </Field>
        <ErrorBox error={a.error} />
        <button className="button primary" disabled={a.busy}>
          {a.busy ? <Spinner /> : "변경하고 시작하기"}
        </button>
      </form>
    </div>
  );
}
