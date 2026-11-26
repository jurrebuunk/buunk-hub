// Talks to the hub's own server. The session cookie goes along by itself;
// there are no tokens here.

export interface Me {
  sub: string;
  name: string;
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
