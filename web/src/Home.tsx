import { Badge, Icon, Spinner } from "@flatwork-ui/react";
import type { App, Me } from "./api";
import { AppIcon } from "./AppIcon";
import { CloudflareLogo, cloudflareOrange, cloudflareTint } from "./cloudflare";
import { joinNames, type Messages } from "./i18n";
import { Link } from "./router";
import { Picture } from "./Shell";
import type { Reach } from "./warp";

// Home (design: Main.dc.html): every app the person has a role for. Apps
// behind WARP show as locked until this browser can reach them.

export function Home({ me, m, apps, reach }: { me: Me; m: Messages; apps: App[]; reach: Record<string, Reach> }) {
  const internal = apps.filter((a) => a.internal);
  const ready = apps.filter((a) => reach[a.id] === "ok").length;
  const checking = internal.some((a) => (reach[a.id] ?? "checking") === "checking");
  const onWarp = internal.some((a) => reach[a.id] === "ok");
  const firstName = me.name.split(" ")[0] || me.name;

  return (
    <>
      <header style={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 16 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
          <Link href="/settings" aria-label={m.changePicture} title={m.changePicture} style={{ position: "relative", display: "block", textDecoration: "none" }}>
            <Picture me={me} size={88} />
            <span
              aria-hidden="true"
              style={{
                position: "absolute",
                right: -2,
                bottom: -2,
                display: "grid",
                placeContent: "center",
                width: 30,
                height: 30,
                borderRadius: "50%",
                background: "var(--fw-surface)",
                border: "1px solid var(--fw-border-strong)",
                color: "var(--fw-heading)",
              }}
            >
              <Icon name="photo_camera" size={16} />
            </span>
          </Link>
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <h1 className="fw-t-display">{m.greeting(new Date().getHours(), firstName)}</h1>
            <p className="fw-t-body fw-t-muted">{apps.length ? m.appCount(apps.length) : m.noApps}</p>
          </div>
        </div>
      </header>

      {/* Only when something needs WARP and this browser can't reach it;
          when everything works there is nothing to say. */}
      {internal.length > 0 && !checking && !onWarp && (
        <section
          aria-labelledby="warp-title"
          className="fw-card"
          style={{
            flexDirection: "row",
            flexWrap: "wrap",
            alignItems: "center",
            gap: 16,
            padding: "16px 20px",
            border: `2px dotted ${cloudflareOrange}`,
            background: cloudflareTint,
          }}
        >
          <CloudflareLogo size={56} />
          <div style={{ flex: "1 1 280px", minWidth: 0, display: "flex", flexDirection: "column", gap: 2 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
              <h2 className="fw-t-heading" id="warp-title">
                {m.warpTitle}
              </h2>
              <Badge style={{ background: "var(--fw-surface)", borderColor: cloudflareOrange }}>
                <span className="fw-dot" style={{ background: cloudflareOrange }} />
                {m.notConnected}
              </Badge>
            </div>
            <p className="fw-t-body" style={{ color: "var(--fw-text)" }}>
              {m.offWarp(joinNames(internal.map((a) => a.name), m.and))}
            </p>
          </div>
          <Link
            className="fw-btn fw-btn-sm"
            href="/warp"
            style={{ background: cloudflareOrange, borderColor: cloudflareOrange, color: "#1b1818", fontWeight: 500 }}
          >
            {m.howToConnect}
            <Icon name="arrow_forward" />
          </Link>
        </section>
      )}

      {apps.length > 0 && (
        <section aria-labelledby="apps-heading" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between", gap: 8 }}>
            <h2 className="fw-t-heading" id="apps-heading">
              {m.yourApps}
            </h2>
            <span className="fw-t-caption">{m.readyNow(ready, apps.length)}</span>
          </div>
          <ul
            style={{
              listStyle: "none",
              margin: 0,
              padding: 0,
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(min(300px, 100%), 1fr))",
              gap: 16,
            }}
          >
            {apps.map((a) => (
              <li key={a.id} style={{ display: "flex" }}>
                <AppCard app={a} reach={reach[a.id] ?? (a.internal ? "checking" : "ok")} m={m} />
              </li>
            ))}
          </ul>
        </section>
      )}
    </>
  );
}

const card = {
  flex: 1,
  display: "flex",
  flexDirection: "column",
  gap: 20,
  minHeight: 210,
  padding: 20,
  borderRadius: "var(--fw-r-lg)",
} as const;

function AppCard({ app, reach, m }: { app: App; reach: Reach; m: Messages }) {
  if (reach === "ok") {
    return (
      <a
        href={app.url}
        aria-label={m.openApp(app.name)}
        style={{
          ...card,
          border: "1px solid var(--fw-border)",
          background: "var(--fw-surface)",
          color: "var(--fw-text)",
          textDecoration: "none",
        }}
      >
        <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
          <AppIcon app={app} size={44} />
          <Badge tone="success" dot>
            {m.ready}
          </Badge>
        </div>
        <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 4 }}>
          <span className="fw-t-heading" style={{ fontSize: 18 }}>
            {app.name}
          </span>
          <span className="fw-t-body fw-t-muted">{app.description}</span>
        </div>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 12,
            paddingTop: 14,
            borderTop: "1px solid var(--fw-border)",
          }}
        >
          <span className="fw-t-caption" style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
            {app.internal ? <CloudflareLogo /> : <Icon name="public" size={16} />}
            {app.internal ? m.internalOverWarp : m.public}
          </span>
          <span style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--fw-heading)", fontSize: 13, fontWeight: 500 }}>
            {m.open}
            <Icon name="arrow_outward" size={16} />
          </span>
        </div>
      </a>
    );
  }

  return (
    <div style={{ ...card, border: "1px dashed var(--fw-border-strong)", background: "var(--fw-bg)" }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
        <AppIcon app={app} size={44} dimmed />
        {reach === "checking" ? (
          <Badge>
            <Spinner /> {m.checking}
          </Badge>
        ) : (
          <Badge style={{ borderColor: cloudflareOrange }}>
            <CloudflareLogo size={14} />
            {m.lockedNeedsWarp}
          </Badge>
        )}
      </div>
      <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 4 }}>
        <span className="fw-t-heading" style={{ fontSize: 18, color: "var(--fw-text)" }}>
          {app.name}
        </span>
        <span className="fw-t-body fw-t-muted">{app.description}</span>
      </div>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          paddingTop: 14,
          borderTop: "1px dashed var(--fw-border-strong)",
        }}
      >
        <span className="fw-t-caption" style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
          <CloudflareLogo />
          {m.internalOverWarp}
        </span>
        {reach === "blocked" && (
          <Link href="/warp" style={{ fontSize: 13, fontWeight: 500 }}>
            {m.connectWarp}
          </Link>
        )}
      </div>
    </div>
  );
}
