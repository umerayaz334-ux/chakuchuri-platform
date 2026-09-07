import { type FormEvent, useState } from "react";
import { Languages } from "lucide-react";
import { apiPost } from "./api";
import { services } from "./constants";
import type { PlatformLanguage } from "./preferences";
import type { Blueprint, Foundation, Session } from "./types";
import { messageFromError } from "./utils";
import "./public-home.css";

const JOURNEY = [
  { step: "01", title: "Quote", detail: "Send specs and get priced." },
  { step: "02", title: "Deposit", detail: "Confirm with transfer proof." },
  { step: "03", title: "Production", detail: "Track stages on the floor." },
  { step: "04", title: "Shipping", detail: "Courier rates and dispatch." }
] as const;

const APP_GLIMPSES = [
  {
    id: "orders",
    name: "Orders",
    blurb: "Stage progress from cut to pack.",
    rows: [
      { label: "Chef knife set", meta: "Cutting · 62%", tone: "live" },
      { label: "Hunting blades", meta: "Polishing · 81%", tone: "live" },
      { label: "Kitchen shears", meta: "Packed", tone: "done" }
    ]
  },
  {
    id: "payments",
    name: "Payments",
    blurb: "Proofs, confirmations and balance.",
    rows: [
      { label: "ABC Export House", meta: "Waiting · Rs 1,000", tone: "warn" },
      { label: "Ledger LED-1064", meta: "Confirmed · Cr Rs 100", tone: "done" },
      { label: "Balance due", meta: "Rs 13,340", tone: "live" }
    ]
  },
  {
    id: "messages",
    name: "Messages",
    blurb: "Live chat with the factory desk.",
    rows: [
      { label: "Quote photos received", meta: "Just now", tone: "live" },
      { label: "Deposit proof uploaded", meta: "12 min", tone: "done" },
      { label: "Shipping window confirmed", meta: "Today", tone: "done" }
    ]
  },
  {
    id: "directory",
    name: "Directory",
    blurb: "Wazirabad makers and contacts.",
    rows: [
      { label: "Edge Works", meta: "Cutlery · Call / WhatsApp", tone: "live" },
      { label: "Steel Craft Co.", meta: "Blades · Wazirabad", tone: "done" },
      { label: "Forge & Finish", meta: "Tools · Map pin", tone: "done" }
    ]
  }
] as const;

export function PublicEntry({
  blueprint: _blueprint,
  foundation: _foundation,
  language,
  allowLanguageChoice,
  notice,
  onLanguageChange,
  onSession,
  onNotice
}: {
  blueprint: Blueprint;
  foundation: Foundation;
  language: PlatformLanguage;
  allowLanguageChoice: boolean;
  notice: string;
  onLanguageChange: (language: PlatformLanguage) => void;
  onSession: (session: Session) => void;
  onNotice: (notice: string) => void;
}) {
  const [activeApp, setActiveApp] = useState<(typeof APP_GLIMPSES)[number]["id"]>("orders");
  const glimpse = APP_GLIMPSES.find((app) => app.id === activeApp) || APP_GLIMPSES[0];

  return (
    <main className="cc-site">
      <section className="cc-site-hero" aria-label="ChakuChuri homepage">
        <div className="cc-site-hero-media" aria-hidden="true" />
        <div className="cc-site-hero-shade" aria-hidden="true" />

        <header className="cc-site-top">
          <a className="cc-site-mark" href="/">
            <span>CC</span>
            ChakuChuri.pk
          </a>
          <nav className="cc-site-nav-links" aria-label="Site">
            <a href="#apps">Apps</a>
            <a href="#services">Services</a>
            <a href="/directory">Directory</a>
            {allowLanguageChoice && (
              <div aria-label="Language" className="cc-site-lang" role="group">
                <Languages aria-hidden size={15} />
                <button aria-pressed={language === "en"} className={language === "en" ? "active" : ""} onClick={() => onLanguageChange("en")} type="button">
                  EN
                </button>
                <button aria-pressed={language === "ur"} className={language === "ur" ? "active" : ""} onClick={() => onLanguageChange("ur")} type="button">
                  <span dir="rtl" lang="ur">
                    اردو
                  </span>
                </button>
              </div>
            )}
          </nav>
        </header>

        <div className="cc-site-hero-copy">
          <p className="cc-site-brand">ChakuChuri.pk</p>
          <h1>Export control for blade makers.</h1>
          <p className="cc-site-lede">Quotes, production, shipping and payments in one portal built for Wazirabad exporters.</p>
          <div className="cc-site-ctas">
            <a className="cc-site-cta primary" href="#portal">
              Sign in
            </a>
            <a
              className="cc-site-cta"
              href="#portal"
              onClick={() => document.getElementById("signup-tab")?.click()}
            >
              Create account
            </a>
          </div>
        </div>
      </section>

      <section className="cc-site-apps" id="apps" aria-label="App glimpses">
        <header>
          <span>Inside the portal</span>
          <h2>A glimpse of the apps.</h2>
        </header>
        <div className="cc-site-apps-switch" role="tablist" aria-label="Portal apps">
          {APP_GLIMPSES.map((app) => (
            <button
              aria-selected={activeApp === app.id}
              className={activeApp === app.id ? "active" : ""}
              key={app.id}
              onClick={() => setActiveApp(app.id)}
              role="tab"
              type="button"
            >
              {app.name}
            </button>
          ))}
        </div>
        <article className="cc-site-app-frame" role="tabpanel">
          <div className="cc-site-app-chrome" aria-hidden="true">
            <i />
            <i />
            <i />
            <em>{glimpse.name}</em>
          </div>
          <div className="cc-site-app-body">
            <div className="cc-site-app-side">
              <b>CC</b>
              {APP_GLIMPSES.map((app) => (
                <span className={app.id === glimpse.id ? "is-active" : ""} key={app.id}>
                  {app.name}
                </span>
              ))}
            </div>
            <div className="cc-site-app-main">
              <header>
                <strong>{glimpse.name}</strong>
                <small>{glimpse.blurb}</small>
              </header>
              <ul>
                {glimpse.rows.map((row) => (
                  <li key={row.label}>
                    <div>
                      <strong>{row.label}</strong>
                      <small>{row.meta}</small>
                    </div>
                    <em className={row.tone}>{row.tone === "warn" ? "Pending" : row.tone === "live" ? "Live" : "Done"}</em>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </article>
      </section>

      <section className="cc-site-portal" id="portal">
        <div className="cc-site-portal-intro">
          <span>Portal</span>
          <h2>Sign in to run your export desk.</h2>
          <p>Customers manage quotes and proofs. Company staff confirm payments, production and shipping.</p>
        </div>
        <AuthPanel onNotice={onNotice} onSession={onSession} />
      </section>

      <section className="cc-site-services" id="services">
        <header>
          <span>From quote to dispatch</span>
          <h2>One path for every order.</h2>
        </header>
        <ol className="cc-site-journey">
          {JOURNEY.map((item) => (
            <li key={item.step}>
              <em>{item.step}</em>
              <strong>{item.title}</strong>
              <p>{item.detail}</p>
            </li>
          ))}
        </ol>
      </section>

      <footer className="cc-site-footer" id="support">
        <strong>ChakuChuri.pk</strong>
        <div>
          <a href="/directory">Directory</a>
          <a href="#portal">Portal</a>
          <a href="mailto:support@chakuchuri.pk">Support</a>
        </div>
      </footer>

      {notice && <div className="cc-toast">{notice}</div>}
    </main>
  );
}

function AuthPanel({ onSession, onNotice }: { onSession: (session: Session) => void; onNotice: (notice: string) => void }) {
  const [mode, setMode] = useState<"login" | "signup">("login");
  const [busy, setBusy] = useState(false);
  const [login, setLogin] = useState({ email: "admin@chakuchuri.pk", password: "admin123" });
  const [signup, setSignup] = useState({
    companyName: "",
    contactName: "",
    email: "",
    phone: "",
    country: "Pakistan",
    password: "",
    services
  });

  const submitLogin = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      onSession(await apiPost<Session>("/api/auth/login", login));
    } catch (error) {
      onNotice(messageFromError(error, "Sign in failed."));
    } finally {
      setBusy(false);
    }
  };

  const submitSignup = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      const payload = await apiPost<{ session: Session }>("/api/auth/signup", signup);
      onSession(payload.session);
    } catch (error) {
      onNotice(messageFromError(error, "Signup failed."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <article className="cc-site-auth">
      <div className="cc-site-auth-tabs">
        <button className={mode === "login" ? "active" : ""} onClick={() => setMode("login")} type="button">
          Login
        </button>
        <button className={mode === "signup" ? "active" : ""} id="signup-tab" onClick={() => setMode("signup")} type="button">
          Signup
        </button>
      </div>
      {mode === "login" ? (
        <form className="cc-site-form" onSubmit={submitLogin}>
          <header>
            <span>Secure access</span>
            <h3>Welcome back</h3>
          </header>
          <label>
            Email
            <input onChange={(event) => setLogin((current) => ({ ...current, email: event.target.value }))} type="email" value={login.email} />
          </label>
          <label>
            Password
            <input onChange={(event) => setLogin((current) => ({ ...current, password: event.target.value }))} type="password" value={login.password} />
          </label>
          <div className="cc-site-demo">
            <button onClick={() => setLogin({ email: "admin@chakuchuri.pk", password: "admin123" })} type="button">
              Admin demo
            </button>
            <button onClick={() => setLogin({ email: "customer@chakuchuri.pk", password: "customer123" })} type="button">
              Customer demo
            </button>
          </div>
          <button className="cc-site-submit" disabled={busy} type="submit">
            Sign in
          </button>
        </form>
      ) : (
        <form className="cc-site-form" onSubmit={submitSignup}>
          <header>
            <span>Customer onboarding</span>
            <h3>Create account</h3>
          </header>
          <div className="cc-site-form-grid">
            <label>
              Company
              <input required onChange={(event) => setSignup((current) => ({ ...current, companyName: event.target.value }))} value={signup.companyName} />
            </label>
            <label>
              Contact
              <input onChange={(event) => setSignup((current) => ({ ...current, contactName: event.target.value }))} value={signup.contactName} />
            </label>
            <label>
              Email
              <input required type="email" onChange={(event) => setSignup((current) => ({ ...current, email: event.target.value }))} value={signup.email} />
            </label>
            <label>
              Phone
              <input onChange={(event) => setSignup((current) => ({ ...current, phone: event.target.value }))} value={signup.phone} />
            </label>
            <label>
              Country
              <input onChange={(event) => setSignup((current) => ({ ...current, country: event.target.value }))} value={signup.country} />
            </label>
            <label>
              Password
              <input minLength={6} required type="password" onChange={(event) => setSignup((current) => ({ ...current, password: event.target.value }))} value={signup.password} />
            </label>
          </div>
          <div className="cc-site-services-pick">
            {services.map((service) => (
              <button
                className={signup.services.includes(service) ? "selected" : ""}
                key={service}
                onClick={() =>
                  setSignup((current) => ({
                    ...current,
                    services: current.services.includes(service) ? current.services.filter((item) => item !== service) : [...current.services, service]
                  }))
                }
                type="button"
              >
                {service}
              </button>
            ))}
          </div>
          <button className="cc-site-submit" disabled={busy} type="submit">
            Create portal
          </button>
        </form>
      )}
    </article>
  );
}
