import { useCallback, useEffect, useState } from "react";
import type { App } from "./api";

// Whether internal apps can be reached from this browser right now. A
// no-cors fetch of the app's root only resolves when the name resolves (on
// WARP) and Gateway lets this person through; the response itself is
// opaque and ignored. Nothing is sent to the hub.

export type Reach = "checking" | "ok" | "blocked";

const timeoutMs = 1500;

async function reachable(url: string): Promise<boolean> {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    await fetch(new URL("/", url), { mode: "no-cors", cache: "no-store", credentials: "omit", signal: ctrl.signal });
    return true;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

/** Reach per app id; public apps are always "ok". */
export function useReach(apps: App[] | undefined): { reach: Record<string, Reach>; recheck: () => void; checkedAt: number } {
  const [reach, setReach] = useState<Record<string, Reach>>({});
  const [round, setRound] = useState(0);
  const [checkedAt, setCheckedAt] = useState(0);

  useEffect(() => {
    if (!apps) return;
    let cancelled = false;
    setReach(Object.fromEntries(apps.map((a) => [a.id, a.internal ? "checking" : "ok"])));
    Promise.all(
      apps
        .filter((a) => a.internal)
        .map(async (a) => {
          const ok = await reachable(a.url);
          if (!cancelled) setReach((r) => ({ ...r, [a.id]: ok ? "ok" : "blocked" }));
        }),
    ).then(() => !cancelled && setCheckedAt(Date.now()));
    return () => {
      cancelled = true;
    };
  }, [apps, round]);

  // Coming back to the tab (after switching WARP on) checks again.
  useEffect(() => {
    const onVisible = () => document.visibilityState === "visible" && setRound((n) => n + 1);
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, []);

  const recheck = useCallback(() => setRound((n) => n + 1), []);
  return { reach, recheck, checkedAt };
}
