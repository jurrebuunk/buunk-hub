// Talks to the hub's own server. The session cookie goes along by itself;
// there are no tokens here.

export interface Me {
  sub: string;
  name: string;
  username: string;
  email: string;
  picture: string;
  lang: string;
}

export interface App {
  id: string;
  name: string;
  url: string;
  icon: string;
  description: string;
  internal: boolean;
}

/** Sends the browser to sign in, then back to where it is now. */
export function signIn(): never {
  const next = location.pathname + location.search;
  location.assign("/auth/login?next=" + encodeURIComponent(next));
  throw new Error("signing in");
}

export async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (res.status === 401) signIn();
  if (!res.ok) throw new Error(`${path}: ${res.status}`);
  return res.json() as Promise<T>;
}

export interface QuickLink {
  id: number;
  title: string;
  url: string;
}

/** A refused change; code is the server's error code ("format", …). */
export class ApiError extends Error {
  constructor(public code: string) {
    super(code);
  }
}

/** Sends a change. Same origin, so the browser adds the Origin header the
 *  server checks; the session cookie goes along by itself. */
export async function send<T = void>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body instanceof FormData || body === undefined ? {} : { "Content-Type": "application/json" },
    body: body instanceof FormData ? body : body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 401) signIn();
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new ApiError(typeof err.error === "string" ? err.error : "other");
  }
  return (res.status === 204 ? undefined : await res.json()) as T;
}

/** One of the person's own pages; refused is the header that stops the
 *  site from being framed ("" when it is embedded). */
export interface HubPage {
  id: number;
  title: string;
  url: string;
  refused: string;
}
