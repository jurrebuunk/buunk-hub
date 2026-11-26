import { useEffect, useState } from "react";
import { Button, EmptyState, FlatworkRoot, Icon, Spinner } from "@flatwork-ui/react";
import { getJSON, type App as HubApp, type HubPage, type Me } from "./api";
import { Home } from "./Home";
import { Settings } from "./Settings";
import { isLang, pickLang, t, type Lang } from "./i18n";
import { PageDialog, PageView } from "./Pages";
import { WelcomeDialog } from "./Profile";
import { navigate, usePath } from "./router";
import { Shell } from "./Shell";
import { WarpGuide } from "./WarpGuide";
import { useReach } from "./warp";

type State = { kind: "loading" } | { kind: "failed" } | { kind: "ready"; me: Me; apps: HubApp[]; pages: HubPage[] };

/** The page id in /pages/<id>, or undefined. */
function pageID(path: string): number | undefined {
  const match = /^\/pages\/(\d+)$/.exec(path);
  return match ? Number(match[1]) : undefined;
}

export function App() {
  const [state, setState] = useState<State>({ kind: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [creating, setCreating] = useState(false);
  // The first-visit dialog: by itself until finished or skipped, and from
  // Home's "Finish your profile" link.
  const [welcoming, setWelcoming] = useState(false);
  const path = usePath();

  useEffect(() => {
    let cancelled = false;
    Promise.all([getJSON<Me>("/api/me"), getJSON<HubApp[]>("/api/apps"), getJSON<HubPage[]>("/api/pages")]).then(
      ([me, apps, pages]) => {
        if (cancelled) return;
        setState({ kind: "ready", me, apps, pages });
        if (!me.welcomed) setWelcoming(true);
      },
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
  const openPage = pageID(path);
  const ownPage = state.kind === "ready" && openPage !== undefined ? state.pages.find((p) => p.id === openPage) : undefined;
  const pageTitle =
    path === "/warp"
      ? m.guideTitle
      : path.startsWith("/settings")
        ? m.settings
        : openPage !== undefined
          ? (ownPage?.title ?? m.myPages)
          : m.home;
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
        <Shell me={state.me} m={m} path={path} pages={state.pages} onNewPage={() => setCreating(true)}>
          {openPage !== undefined ? (
            <PageView m={m} id={openPage} pages={state.pages} setPages={(pages) => setState({ ...state, pages })} />
          ) : path === "/warp" ? (
            <WarpGuide m={m} apps={state.apps} reach={reach} recheck={recheck} />
          ) : path === "/settings" || path === "/settings/start" ? (
            <Settings me={state.me} m={m} path={path} setMe={(me) => setState({ ...state, me })} />
          ) : (
            <Home
              me={state.me}
              m={m}
              apps={state.apps}
              reach={reach}
              pages={state.pages}
              onNewPage={() => setCreating(true)}
              onFinishProfile={() => setWelcoming(true)}
            />
          )}
          {welcoming && (
            <WelcomeDialog m={m} me={state.me} setMe={(me) => setState({ ...state, me })} onClose={() => setWelcoming(false)} />
          )}
          {creating && (
            <PageDialog
              m={m}
              mode="new"
              onClose={() => setCreating(false)}
              onSaved={(p) => {
                setCreating(false);
                setState({ ...state, pages: [...state.pages, p] });
                navigate(`/pages/${p.id}`);
              }}
            />
          )}
        </Shell>
      )}
    </FlatworkRoot>
  );
}
