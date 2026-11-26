// Every user-visible string, in English and Dutch. Until sign-in exists
// (milestone 2) the language comes from the browser; then it comes from the
// token's locale claim.
const messages = {
  en: {
    title: "buunk hub",
    comingSoon: "Under construction",
    comingSoonText: "Your apps, pages and settings will be here soon.",
  },
  nl: {
    title: "buunk hub",
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

export function t(lang: Lang): Messages {
  return messages[lang];
}
