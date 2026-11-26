import { Badge, Card, Icon, Spinner, type IconName } from "@flatwork-ui/react";
import type { App, Me } from "./api";
import { joinNames, type Messages } from "./i18n";
import { Link } from "./router";
import { Picture } from "./Shell";
import type { Reach } from "./warp";

// Home (design: Main.dc.html): every app the person has a role for. Apps
// behind WARP show as locked until this browser can reach them.

const tile = {
  display: "grid",
  placeContent: "center",
  width: 56,
  height: 56,
  borderRadius: "var(--fw-r-md)",
} as const;

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
          <Picture me={me} size={88} />
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <h1 className="fw-t-display">{m.greeting(new Date().getHours(), firstName)}</h1>
            <p className="fw-t-body fw-t-muted">{apps.length ? m.appCount(apps.length) : m.noApps}</p>
          </div>
        </div>
      </header>

      {internal.length > 0 && (
        <Card
          aria-labelledby="warp-title"
          style={{ flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 16, padding: "16px 20px" }}
        >
          <span
            aria-hidden="true"
            style={{
              ...tile,
              width: 48,
              height: 48,
              background: "var(--fw-surface-2)",
              border: "1px solid var(--fw-border-strong)",
              color: "var(--fw-warning)",
            }}
          >
            <Icon name="cloud" size={28} />
          </span>
          <div style={{ flex: "1 1 280px", minWidth: 0, display: "flex", flexDirection: "column", gap: 2 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
              <h2 className="fw-t-heading" id="warp-title">
                {m.warpTitle}
              </h2>
              {checking ? (
                <Badge>
                  <Spinner /> {m.checking}
                </Badge>
              ) : onWarp ? (
                <Badge tone="success" dot>
                  {m.connected}
                </Badge>
              ) : (
                <Badge tone="warning" dot>
                  {m.notConnected}
                </Badge>
              )}
            </div>
            {!checking && (
              <p className="fw-t-body fw-t-muted" aria-live="polite">
                {onWarp ? m.onWarp : m.offWarp(joinNames(internal.map((a) => a.name), m.and))}
              </p>
            )}
          </div>
          {!checking && !onWarp && (
            <Link className="fw-btn fw-btn-secondary fw-btn-sm" href="/warp">
              {m.howToConnect}
              <Icon name="arrow_forward" />
            </Link>
          )}
          {!checking && onWarp && <span className="fw-t-caption">{m.checkedJustNow}</span>}
        </Card>
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
  const icon = app.icon as IconName;
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
          <span
            aria-hidden="true"
            style={{ ...tile, background: "var(--fw-surface-2)", border: "1px solid var(--fw-border-strong)", color: "var(--fw-accent)" }}
          >
            <Icon name={icon} size={30} />
          </span>
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
            <Icon name={app.internal ? "vpn_lock" : "public"} size={16} />
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
        <span
          aria-hidden="true"
          style={{
            ...tile,
            position: "relative",
            background: "var(--fw-surface)",
            border: "1px solid var(--fw-border)",
            color: "var(--fw-muted)",
          }}
        >
          <Icon name={icon} size={30} />
          <span
            style={{
              position: "absolute",
              right: -8,
              bottom: -8,
              display: "grid",
              placeContent: "center",
              width: 26,
              height: 26,
              borderRadius: "50%",
              background: "var(--fw-warning-bg)",
              border: "1px solid var(--fw-warning-border)",
              color: "var(--fw-warning)",
            }}
          >
            <Icon name="lock" size={15} />
          </span>
        </span>
        {reach === "checking" ? (
          <Badge>
            <Spinner /> {m.checking}
          </Badge>
        ) : (
          <Badge tone="warning">
            <Icon name="vpn_lock" size={14} />
            {m.needsWarp}
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
          <Icon name="vpn_lock" size={16} />
          {m.internal}
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
