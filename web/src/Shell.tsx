import type { ReactNode } from "react";
import { Avatar, Button, Icon, NavItem, SideNav } from "@flatwork-ui/react";
import type { Me } from "./api";
import type { Messages } from "./i18n";
import { Link } from "./router";

export function initials(name: string): string {
  return name
    .split(/[\s@.]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join("");
}

export function Picture({ me, size }: { me: Me; size?: number }) {
  const style = size ? { width: size, height: size, fontSize: size * 0.32, borderWidth: 2 } : undefined;
  return (
    <Avatar aria-hidden="true" style={style}>
      {me.picture ? <img src={me.picture} alt="" /> : initials(me.name || me.email)}
    </Avatar>
  );
}

/** The sidebar and the page next to it (designs: every hub page). */
export function Shell({ me, m, path, children }: { me: Me; m: Messages; path: string; children: ReactNode }) {
  return (
    <div style={{ minHeight: "100vh", padding: 16, display: "flex", flexWrap: "wrap", gap: 16, alignItems: "stretch" }}>
      <SideNav aria-label={m.nav} style={{ flex: "1 1 248px" }}>
        <Link
          href="/"
          style={{
            display: "flex",
            alignItems: "center",
            gap: 10,
            padding: "6px 10px 14px",
            fontWeight: 600,
            color: "var(--fw-heading)",
            textDecoration: "none",
          }}
        >
          <span className="fw-logo-mark" aria-hidden="true">
            <Icon name="hub" />
          </span>
          {m.title}
        </Link>
        <NavItem as={Link} href="/" current={path === "/"} icon={<Icon name="home" />}>
          {m.home}
        </NavItem>
        <div
          style={{
            marginTop: "auto",
            padding: "12px 4px 0",
            borderTop: "1px solid var(--fw-border)",
            display: "flex",
            alignItems: "center",
            gap: 10,
          }}
        >
          <Picture me={me} />
          <span style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column" }}>
            <span className="fw-t-label">{me.name}</span>
            <span className="fw-t-caption" style={{ overflow: "hidden", textOverflow: "ellipsis" }}>
              {me.email}
            </span>
          </span>
          {/* A plain form: the browser sends Origin with it, which the server
              checks, and the logout redirect to Keycloak just happens. */}
          <form method="post" action="/auth/logout">
            <Button type="submit" variant="ghost" size="sm" iconOnly aria-label={m.signOut} title={m.signOut}>
              <Icon name="logout" />
            </Button>
          </form>
        </div>
      </SideNav>
      <main
        style={{ flex: "999 1 560px", minWidth: 0, display: "flex", flexDirection: "column", gap: 28, padding: "24px 16px" }}
      >
        {children}
      </main>
    </div>
  );
}
