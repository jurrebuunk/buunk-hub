import { useEffect, useState, type ReactNode } from "react";
import { Avatar, Button, Icon, NavItem, SideNav, VisuallyHidden } from "@flatwork-ui/react";
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

// Folded or not is a convenience for this browser only; storage may be
// unavailable (private windows), then it just starts unfolded.
const collapsedKey = "hub.sidebar.collapsed";

function loadCollapsed(): boolean {
  try {
    return localStorage.getItem(collapsedKey) === "1";
  } catch {
    return false;
  }
}

function saveCollapsed(v: boolean) {
  try {
    localStorage.setItem(collapsedKey, v ? "1" : "0");
  } catch {
    // not remembered, that's all
  }
}

// Phones always get the folded bar; it sits above the page there.
const narrowQuery = "(max-width: 720px)";

function useNarrow(): boolean {
  const [narrow, setNarrow] = useState(() => matchMedia(narrowQuery).matches);
  useEffect(() => {
    const mq = matchMedia(narrowQuery);
    const update = () => setNarrow(mq.matches);
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, []);
  return narrow;
}

/** The sidebar and the page next to it (designs: every hub page). */
export function Shell({ me, m, path, children }: { me: Me; m: Messages; path: string; children: ReactNode }) {
  const [stored, setCollapsed] = useState(loadCollapsed);
  const narrow = useNarrow();
  const collapsed = stored || narrow;
  const toggle = () => {
    setCollapsed((c) => {
      saveCollapsed(!c);
      return !c;
    });
  };
  // Folded, a name is only read out and shown as a tooltip.
  const label = (text: string) => (collapsed ? <VisuallyHidden>{text}</VisuallyHidden> : text);

  return (
    <div className="hub-layout">
      <SideNav id="hub-sidebar" aria-label={m.nav} className={collapsed ? "hub-side is-collapsed" : "hub-side"}>
        <Link href="/" className="hub-brand" title={collapsed ? m.title : undefined}>
          <span className="fw-logo-mark" aria-hidden="true">
            <Icon name="hub" />
          </span>
          {label(m.title)}
        </Link>
        <NavItem
          as={Link}
          href="/"
          current={path === "/"}
          icon={<Icon name="home" />}
          title={collapsed ? m.home : undefined}
        >
          {label(m.home)}
        </NavItem>
        <span className="fw-group-label hub-group">{label(m.you)}</span>
        <NavItem
          as={Link}
          href="/settings"
          current={path.startsWith("/settings")}
          icon={<Icon name="settings" />}
          title={collapsed ? m.settings : undefined}
        >
          {label(m.settings)}
        </NavItem>
        {/* The start page is the server's own page, not part of the app. */}
        <NavItem href="/start" icon={<Icon name="travel_explore" />} title={collapsed ? m.startPage : undefined}>
          {label(m.startPage)}
        </NavItem>
        {/* Above the profile, a row like the nav items, its icon in the same
            column open or folded. Phones always get the folded bar. */}
        {!narrow && (
          <button
            type="button"
            className="fw-nav-item hub-toggle"
            aria-expanded={!collapsed}
            aria-controls="hub-sidebar"
            aria-label={collapsed ? m.expandSidebar : m.collapseSidebar}
            title={collapsed ? m.expandSidebar : undefined}
            onClick={toggle}
          >
            <Icon name={collapsed ? "left_panel_open" : "left_panel_close"} />
            {!collapsed && (
              <span className="hub-toggle-text" aria-hidden="true">
                {m.collapseSidebar}
              </span>
            )}
          </button>
        )}
        <div className="hub-account">
          <span title={collapsed ? `${me.name} · ${me.email}` : undefined}>
            <Picture me={me} />
          </span>
          {!collapsed && (
            <span style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column" }}>
              <span className="fw-t-label">{me.name}</span>
              <span className="fw-t-caption" style={{ overflow: "hidden", textOverflow: "ellipsis" }}>
                {me.email}
              </span>
            </span>
          )}
          {/* A plain form: the browser sends Origin with it, which the server
              checks, and the logout redirect to Keycloak just happens. */}
          <form method="post" action="/auth/logout">
            <Button type="submit" variant="ghost" size="sm" iconOnly aria-label={m.signOut} title={m.signOut}>
              <Icon name="logout" />
            </Button>
          </form>
        </div>
      </SideNav>
      <main className="hub-main">{children}</main>
    </div>
  );
}
