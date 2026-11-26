import { useEffect, useState } from "react";
import { Button, EmptyState, FlatworkRoot, Icon, Spinner } from "@flatwork-ui/react";
import { getJSON, type App as HubApp, type Me } from "./api";
import { Home } from "./Home";
import { isLang, pickLang, t, type Lang } from "./i18n";
import { usePath } from "./router";
import { Shell } from "./Shell";
import { WarpGuide } from "./WarpGuide";
import { useReach } from "./warp";

type State = { kind: "loading" } | { kind: "failed" } | { kind: "ready"; me: Me; apps: HubApp[] };

export function App() {
  const [state, setState] = useState<State>({ kind: "loading" });
  const [attempt, setAttempt] = useState(0);
  const path = usePath();

  useEffect(() => {
    let cancelled = false;
    Promise.all([getJSON<Me>("/api/me"), getJSON<HubApp[]>("/api/apps")]).then(
      ([me, apps]) => !cancelled && setState({ kind: "ready", me, apps }),
      () => !cancelled && setState({ kind: "failed" }),
    );
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  const apps = state.kind === "ready" ? state.apps : undefined;
  const { reach, recheck } = useReach(apps);

  const lang: Lang = state.kind === "ready" && isLang(state.me.lang) ? state.me.lang : pickLang(navigator.languages);
  const m = t(lang);
  const pageTitle = path === "/warp" ? m.guideTitle : m.home;
  useEffect(() => {
    document.documentElement.lang = lang;
    document.title = `${pageTitle} · ${m.title}`;
  }, [lang, m, pageTitle]);

  return (
    <FlatworkRoot mode="system" style={{ minHeight: "100vh" }}>
      {state.kind === "loading" && (
        <div style={{ minHeight: "100vh", display: "grid", placeItems: "center" }}>
          <Spinner size="lg" label={m.loading} />
        </div>
      )}
      {state.kind === "failed" && (
        <main style={{ minHeight: "100vh", display: "grid", placeItems: "center", padding: 16 }}>
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
        </main>
      )}
      {state.kind === "ready" && (
        <Shell me={state.me} m={m} path={path}>
          {path === "/warp" ? (
            <WarpGuide m={m} apps={state.apps} reach={reach} recheck={recheck} />
          ) : (
            <Home me={state.me} m={m} apps={state.apps} reach={reach} />
          )}
        </Shell>
      )}
    </FlatworkRoot>
  );
}
