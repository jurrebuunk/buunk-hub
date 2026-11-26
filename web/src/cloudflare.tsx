import type { CSSProperties } from "react";
import { siCloudflare } from "simple-icons";

// Cloudflare's brand orange and logo (simple-icons, CC0), for what goes
// through Cloudflare WARP. Orange is only for marks, borders and tints:
// as text it has too little contrast on the light theme.
export const cloudflareOrange = `#${siCloudflare.hex}`;

/** A faint orange wash over the surface; text on it keeps its contrast. */
export const cloudflareTint = `color-mix(in srgb, ${cloudflareOrange} 12%, var(--fw-surface))`;

export function CloudflareLogo({ size = 16, label, style }: { size?: number; label?: string; style?: CSSProperties }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill={cloudflareOrange}
      style={{ flex: "none", ...style }}
      {...(label ? { role: "img", "aria-label": label } : { "aria-hidden": true })}
    >
      {label && <title>{label}</title>}
      <path d={siCloudflare.path} />
    </svg>
  );
}
