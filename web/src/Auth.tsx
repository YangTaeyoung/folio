import { useState } from "react";
import {
  ArrowRight,
  FileCode,
  KeyRound,
  ShieldCheck,
  Layers,
  Share2,
} from "lucide-react";
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
  const [name, setName] = useState("");
  const [org, setOrg] = useState("");
  const [token, setToken] = useState(
    () => new URLSearchParams(location.hash.slice(1)).get("setup") || "",
  );
  const action = useAction();
  return (
    <main className="auth-page">
      <section className="auth-story">
        <Brand />
        <div>
          <h1>
            만든 결과물을,
            <br />
            하나의 작업 공간에.
          </h1>
          <p>
            문서와 작은 웹앱부터 팀의 프로토타입까지.
            <br />내 서버에 게시하고, 필요한 사람에게 공유하세요.
          </p>
          <div className="auth-artifact-stack" aria-hidden="true">
            <div className="auth-artifact">
              <FileCode size={24} />
              <span>Product prototype</span>
              <div className="auth-preview">
                <i />
                <i />
                <i />
              </div>
              <small>HTML · Interactive</small>
            </div>
            <div className="auth-artifact">
              <Layers size={24} />
              <span>Project brief</span>
              <div className="auth-document-lines">
                <i />
                <i />
                <i />
              </div>
              <small>Folio workspace</small>
              <Share2 size={17} />
            </div>
          </div>
        </div>
        <footer>
          <ShieldCheck size={16} />
          우리 서버에 보관하는 팀의 작업 공간
        </footer>
      </section>
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
          <Brand />
          <h2>{setup ? "작업 공간 만들기" : "작업 공간에 로그인"}</h2>
          <p className="subtle">
            {setup
              ? "첫 관리자 계정을 만들고 아티팩트를 모아보세요."
              : "관리자가 생성한 계정으로 로그인해 주세요."}
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
          <Field
            label="비밀번호"
            hint={setup ? "12자 이상으로 설정해 주세요." : undefined}
          >
            <input
              required
              type="password"
              minLength={setup ? 12 : undefined}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={setup ? "new-password" : "current-password"}
            />
          </Field>
          <ErrorBox error={action.error} />
          <button className="button primary auth-submit" disabled={action.busy}>
            {action.busy ? <Spinner /> : setup ? "작업 공간 만들기" : "로그인"}
            <ArrowRight size={18} />
          </button>
          <p className="auth-foot">Folio · MIT 오픈소스 · 내 서버, 내 데이터</p>
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
