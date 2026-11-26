import { useEffect, useState } from "react";
import { Avatar, Brand, Button, EmptyState, FlatworkRoot, Icon, Spinner, TopNav } from "@flatwork-ui/react";
import { getJSON, type Me } from "./api";
import { isLang, pickLang, t, type Lang } from "./i18n";

type State = { kind: "loading" } | { kind: "failed" } | { kind: "ready"; me: Me };

export function App() {
  const [state, setState] = useState<State>({ kind: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    getJSON<Me>("/api/me").then(
      (me) => !cancelled && setState({ kind: "ready", me }),
      () => !cancelled && setState({ kind: "failed" }),
    );
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  const lang: Lang = state.kind === "ready" && isLang(state.me.lang) ? state.me.lang : pickLang(navigator.languages);
  const m = t(lang);
  useEffect(() => {
    document.documentElement.lang = lang;
    document.title = m.title;
  }, [lang, m]);

  return (
    <FlatworkRoot mode="system" style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <TopNav
        brand={
          <Brand href="/" mark="b">
            {m.title}
          </Brand>
        }
        end={state.kind === "ready" && <Account me={state.me} signOut={m.signOut} />}
      />
      <main style={{ flex: 1, display: "grid", placeItems: "center", padding: 16 }}>
        {state.kind === "loading" && <Spinner size="lg" label={m.loading} />}
        {state.kind === "failed" && (
          <EmptyState
            icon={<Icon name="cloud_off" />}
            title={m.loadFailed}
            action={
              <Button
                onClick={() => {
                  setState({ kind: "loading" });
                  setAttempt((n) => n + 1);
                }}
              >
                {m.retry}
              </Button>
            }
          />
        )}
        {state.kind === "ready" && (
          <div style={{ display: "grid", gap: 16, justifyItems: "center" }}>
            <h1 className="fw-t-title">{m.welcome(state.me.name)}</h1>
            <EmptyState icon={<Icon name="construction" />} title={m.comingSoon} description={m.comingSoonText} />
          </div>
        )}
      </main>
    </FlatworkRoot>
  );
}

function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join("");
}

function Account({ me, signOut }: { me: Me; signOut: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
      <Avatar size="sm" aria-hidden="true">
        {me.picture ? <img src={me.picture} alt="" /> : initials(me.name || me.email)}
      </Avatar>
      <span>{me.name}</span>
      {/* A plain form: the browser sends Origin with it, which the server
          checks, and the logout redirect to Keycloak just happens. */}
      <form method="post" action="/auth/logout">
        <Button type="submit" size="sm" leadingIcon={<Icon name="logout" />}>
          {signOut}
        </Button>
      </form>
    </div>
  );
}
