import { useState } from "react";
import { Icon, type IconName } from "@flatwork-ui/react";
import type { App } from "./api";

/** The site's own icon (the server finds it); the catalogue's icon when
 *  the site has none or it can't be fetched. Decorative: the name is next
 *  to it. */
export function AppIcon({ app, size, dimmed }: { app: App; size: number; dimmed?: boolean }) {
  const [failed, setFailed] = useState(false);
  const style = { display: "block", width: size, height: size, opacity: dimmed ? 0.55 : 1, filter: dimmed ? "grayscale(1)" : undefined };
  if (failed) {
    return <Icon name={app.icon as IconName} size={size} aria-hidden="true" style={{ ...style, color: "var(--fw-heading)" }} />;
  }
  return (
    <img
      src={`/api/apps/${encodeURIComponent(app.id)}/favicon`}
      alt=""
      width={size}
      height={size}
      loading="lazy"
      onError={() => setFailed(true)}
      style={{ ...style, objectFit: "contain" }}
    />
  );
}
