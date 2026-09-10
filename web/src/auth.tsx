import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
  type FormEvent,
} from "react";
import {
  ArrowUpRight,
  ArrowRight,
  Eye,
  EyeOff,
  Layers3,
  Sparkles,
  Check,
} from "lucide-react";
import {
  authenticate,
  errorText,
  logout,
  onAuthLost,
  refreshAuth,
  type User,
  ApiError,
} from "./api";
import { ErrorBox, Logo, Spinner } from "./ui";

const AuthContext = createContext<{
  user: User;
  signOut: () => Promise<void>;
} | null>(null);
export const useAuth = () => useContext(AuthContext)!;
export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  async function restore() {
    setLoading(true);
    setError("");
    try {
      setUser((await refreshAuth()).user);
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 401)) setError(errorText(e));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    onAuthLost(() => setUser(null));
    void restore();
  }, []);
  if (loading)
    return (
      <div className="boot">
        <Logo />
        <Spinner />
      </div>
    );
  if (error)
    return (
      <div className="boot">
        <Logo />
        <ErrorBox error={error} retry={restore} />
      </div>
    );
  if (!user) return <AuthPage onLogin={setUser} />;
  return (
    <AuthContext.Provider
      value={{
        user,
        signOut: async () => {
          await logout();
          setUser(null);
        },
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
function AuthPage({ onLogin }: { onLogin: (u: User) => void }) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [visible, setVisible] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    setBusy(true);
    setError("");
    try {
      const result = await authenticate(
        mode,
        String(data.get("nickname")),
        String(data.get("password")),
      );
      onLogin(result.user);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-page">
      <section className="auth-story">
        <Logo />
        <div className="auth-story-main">
          <span className="eyebrow">
            <span className="status-dot" /> A LITTLE EVERY DAY. A LOT OVER TIME.
          </span>
          <h1>
            Make room for
            <br />
            what <em>stays.</em>
          </h1>
          <p>
            Your ideas, your questions, your next big thing.
            <br />A quiet space to turn knowledge into second nature.
          </p>
          <div className="memory-art" aria-hidden="true">
            <div className="orbit orbit-one" />
            <div className="orbit orbit-two" />
            <div className="art-card art-back">
              <Layers3 size={22} />
              <span>Build your knowledge</span>
            </div>
            <div className="art-card art-front">
              <div className="art-label">
                <Sparkles size={15} /> A SMALL MOMENT OF PROGRESS
              </div>
              <strong>
                Learn it. Recall it.
                <br />
                Make it yours.
              </strong>
              <div className="art-progress">
                <span />
                <span />
                <span />
                <span />
                <span />
              </div>
              <div className="art-bottom">
                <span>One step at a time</span>
                <span className="art-check">
                  <Check size={14} />
                </span>
              </div>
            </div>
          </div>
        </div>
        <div className="auth-foot">
          Built for curious minds.
          <ArrowUpRight size={16} />
        </div>
      </section>
      <section className="auth-form-wrap">
        <div className="auth-mobile-logo">
          <Logo />
        </div>
        <form className="auth-form" onSubmit={submit}>
          <span className="eyebrow">YOUR LEARNING SPACE</span>
          <h2>
            {mode === "login" ? "Good to see you." : "Start something lasting."}
          </h2>
          <p>
            {mode === "login"
              ? "Pick up where your curiosity left off."
              : "Your next chapter starts with a little curiosity."}
          </p>
          <label>
            Username
            <input
              name="nickname"
              autoComplete="username"
              autoFocus
              required
              minLength={3}
              maxLength={16}
              placeholder="Your username"
            />
          </label>
          <label>
            Password
            <div className="password-input">
              <input
                name="password"
                type={visible ? "text" : "password"}
                autoComplete={
                  mode === "login" ? "current-password" : "new-password"
                }
                required
                minLength={mode === "register" ? 8 : 1}
                maxLength={128}
                placeholder={
                  mode === "register"
                    ? "At least 8 characters"
                    : "Your password"
                }
              />
              <button
                type="button"
                className="icon-button"
                aria-label={visible ? "Hide password" : "Show password"}
                onClick={() => setVisible(!visible)}
              >
                {visible ? <EyeOff size={18} /> : <Eye size={18} />}
              </button>
            </div>
          </label>
          {error && <ErrorBox error={error} />}
          <button className="button primary full" disabled={busy}>
            {busy
              ? "Just a moment…"
              : mode === "login"
                ? "Sign in"
                : "Create account"}
            <ArrowRight size={18} />
          </button>
          <p className="auth-switch">
            {mode === "login" ? "New around here?" : "Already have a space?"}{" "}
            <button
              type="button"
              className="text-button"
              onClick={() => {
                setMode(mode === "login" ? "register" : "login");
                setError("");
              }}
            >
              {mode === "login" ? "Create an account" : "Sign in"}
            </button>
          </p>
        </form>
        <span className="auth-note">
          <span className="status-dot" /> Your progress, always saved.
        </span>
      </section>
    </div>
  );
}
