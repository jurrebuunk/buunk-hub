// Every user-visible string, in English and Dutch. The server picks the
// language (the token's locale, else Accept-Language) and says it in
// /api/me; before that answers, the browser's language is used.
const messages = {
  en: {
    title: "buunk hub",
    loading: "Loading…",
    loadFailed: "The hub couldn't be reached. Try again in a moment.",
    retry: "Try again",
    signOut: "Sign out",
    welcome: (name: string) => `Welcome, ${name}`,
    comingSoon: "Under construction",
    comingSoonText: "Your apps, pages and settings will be here soon.",
  },
  nl: {
    title: "buunk hub",
    loading: "Laden…",
    loadFailed: "De hub is niet bereikbaar. Probeer het zo opnieuw.",
    retry: "Opnieuw proberen",
    signOut: "Uitloggen",
    welcome: (name: string) => `Welkom, ${name}`,
    comingSoon: "In aanbouw",
    comingSoonText: "Je apps, pagina's en instellingen komen hier binnenkort.",
  },
} as const;

export type Lang = keyof typeof messages;
export type Messages = (typeof messages)[Lang];

export function pickLang(languages: readonly string[]): Lang {
  for (const l of languages) {
    const base = l.toLowerCase().split("-")[0];
    if (base === "nl" || base === "en") return base;
  }
  return "en";
}

export function isLang(s: string): s is Lang {
  return s in messages;
}

export function t(lang: Lang): Messages {
  return messages[lang];
}
