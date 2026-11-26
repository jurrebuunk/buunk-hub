// Every user-visible string, in English and Dutch. The server picks the
// language (the token's locale, else Accept-Language) and says it in
// /api/me; before that answers, the browser's language is used.

const en = {
  title: "buunk hub",
  loading: "Loading…",
  loadFailed: "The hub couldn't be reached. Try again in a moment.",
  retry: "Try again",
  signOut: "Sign out",
  nav: "Hub",
  home: "Home",
  collapseSidebar: "Collapse sidebar",
  expandSidebar: "Expand sidebar",

  you: "You",
  settings: "Settings",
  startPage: "Start page",
  changePicture: "Change your profile picture",
  settingsIntro: "Only you see these.",
  settingsSections: "Settings sections",
  tabProfile: "Profile",
  tabStart: "Start page and quick links",
  pictureTitle: "Profile picture",
  pictureSub: "Shown in the hub and in other buunk apps, like Cloudreve.",
  currentPicture: "Your current picture",
  removePicture: "Remove picture",
  dropTitle: "Drop a picture here",
  dropOr: "or choose a file",
  dropHint: "JPEG, PNG or WebP, up to 5 MB. We crop it to a square and remove photo metadata.",
  uploading: "Uploading…",
  cropTitle: "Frame your picture",
  cropHelp: "Drag to move it; zoom with the slider. It shows round, like here.",
  cropArea: "Picture: drag, or use the arrow keys to move it and + and - to zoom",
  zoom: "Zoom",
  savePicture: "Save picture",
  pictureSaved: "Picture saved. Other apps show it after you sign in there again.",
  pictureRemoved: "Picture removed.",
  accountTitle: "Account",
  accountSub: "Your name and email come from your buunk account.",
  name: "Name",
  email: "Email",
  username: "Username",
  accountHelp: "To change your name or email, ask whoever runs buunk. Forgot your password? Use Forgot password on the sign-in page.",
  startTitle: "Browser start page",
  startSub: "Search, your apps and your quick links on one light page. Set it as your browser's home or new-tab page.",
  startAddress: "Start page address",
  preview: "Preview",
  linksTitle: "Quick links",
  linksSub: "Shown on your start page, in this order.",
  linkCount: (n: number) => (n === 1 ? "1 link" : `${n} links`),
  noLinks: "No quick links yet.",
  addLink: "Add a quick link",
  linkTitle: "Title",
  linkAddress: "Address",
  linkTitlePlaceholder: "Weather",
  linkAddressPlaceholder: "https://buienradar.nl",
  addLinkButton: "Add link",
  save: "Save",
  cancel: "Cancel",
  moveUp: (t: string) => `Move ${t} up`,
  moveDown: (t: string) => `Move ${t} down`,
  edit: (t: string) => `Edit ${t}`,
  remove: (t: string) => `Remove ${t}`,
  errors: {
    "too-large": "That picture is larger than 5 MB.",
    format: "That isn't a JPEG, PNG or WebP picture.",
    dimensions: "That picture is too small or has too many pixels.",
    "invalid-link": "Give a title and an http:// or https:// address.",
    "too-many": "You have the most quick links there can be (50).",
    other: "That didn't work. Try again in a moment.",
  } as Record<string, string>,

  greeting: (hour: number, name: string) =>
    `${hour < 6 ? "Good evening" : hour < 12 ? "Good morning" : hour < 18 ? "Good afternoon" : "Good evening"}, ${name}`,
  appCount: (n: number) => (n === 1 ? "1 app on your buunk account." : `${n} apps on your buunk account.`),
  noApps: "No apps on your buunk account yet. Ask whoever runs buunk for you.",
  yourApps: "Your apps",
  readyNow: (ready: number, all: number) => `${ready} of ${all} ready now`,
  ready: "Ready",
  checking: "Checking…",
  lockedNeedsWarp: "Locked · Needs WARP",
  public: "Public",
  internalOverWarp: "Internal, over WARP",
  open: "Open",
  openApp: (name: string) => `Open ${name}`,
  connectWarp: "Connect WARP",

  warpTitle: "Cloudflare WARP",
  notConnected: "Not connected",
  offWarp: (names: string) => `${names} only open over WARP. Connect it and they unlock here.`,
  howToConnect: "How to connect",
  and: "and",

  guideTitle: "Connect to WARP",
  guideIntro:
    "Some buunk apps only run on our own network. WARP connects your device to it safely, from anywhere. It takes about five minutes, once per device.",
  yourDevice: "Your device",
  platforms: { win: "Windows", mac: "macOS", linux: "Linux", ios: "iPhone and iPad", android: "Android" },
  step1Title: (mobile: boolean) => `Install the ${mobile ? "Cloudflare One Agent app" : "Cloudflare One Client"}`,
  step1Text: "Cloudflare's app, formerly called WARP. It's free.",
  download: (mobile: boolean, platform: string) => (mobile ? `Get it for ${platform}` : `Download for ${platform}`),
  step2Title: "Open it and choose Zero Trust security",
  step2Text: (mobile: boolean): string =>
    mobile
      ? "Accept the privacy policy, then pick Zero Trust security when the app asks what you want to use it for."
      : "On the first screen, pick Zero Trust security. Already used it before? Open the gear icon, then Preferences, Account, Login with Cloudflare Zero Trust.",
  step3Title: "Enter our team name",
  teamName: "Team name",
  copy: "Copy",
  copied: "Copied",
  step4Title: "Sign in with your buunk account",
  step4Text: (mobile: boolean): string =>
    mobile
      ? "A sign-in page opens. Choose buunk, sign in as you do for the hub, then tap Install VPN Profile and allow it."
      : "Your browser opens a sign-in page. Choose buunk, sign in as you do for the hub, then choose Open Cloudflare WARP to finish.",
  step5Title: "Switch it on",
  step5Text: "Flip the switch in the app to Connected, then check here.",
  stillNotConnected: "Not connected yet",
  nowConnected: "Connected",
  checkedFromBrowser: "Checked a moment ago from this browser.",
  checkAgain: "Check again",
  unlocks: "What WARP unlocks",
  unlocked: "Reachable",
  locked: "Not reachable",
  noInternalApps: "None of your apps needs WARP right now.",
  faq: [
    ["Do I have to keep it on?", "No. Switch it on when you want to use an app marked Needs WARP, and off whenever you like."],
    [
      "I signed in but apps stay locked",
      "Check the app says Connected, then press Check again. Still locked? Your account may not have access to that app yet.",
    ],
    ["Who can help me?", "Ask whoever runs buunk for you. Mention which device you're setting up and what step you're on."],
  ] as [string, string][],
};

export type Messages = typeof en;

const nl: Messages = {
  title: "buunk hub",
  loading: "Laden…",
  loadFailed: "De hub is niet bereikbaar. Probeer het zo opnieuw.",
  retry: "Opnieuw proberen",
  signOut: "Uitloggen",
  nav: "Hub",
  home: "Home",
  collapseSidebar: "Zijbalk inklappen",
  expandSidebar: "Zijbalk uitklappen",

  you: "Jij",
  settings: "Instellingen",
  startPage: "Startpagina",
  changePicture: "Je profielfoto wijzigen",
  settingsIntro: "Alleen jij ziet deze.",
  settingsSections: "Onderdelen van de instellingen",
  tabProfile: "Profiel",
  tabStart: "Startpagina en snelle links",
  pictureTitle: "Profielfoto",
  pictureSub: "Te zien in de hub en in andere buunk-apps, zoals Cloudreve.",
  currentPicture: "Je huidige foto",
  removePicture: "Foto verwijderen",
  dropTitle: "Sleep een foto hierheen",
  dropOr: "of kies een bestand",
  dropHint: "JPEG, PNG of WebP, tot 5 MB. We snijden hem vierkant bij en halen de fotogegevens eruit.",
  uploading: "Uploaden…",
  cropTitle: "Kies het beeld",
  cropHelp: "Sleep om te verschuiven; zoom met de schuifbalk. Hij wordt rond getoond, zoals hier.",
  cropArea: "Foto: sleep, of verschuif met de pijltjestoetsen en zoom met + en -",
  zoom: "Zoom",
  savePicture: "Foto opslaan",
  pictureSaved: "Foto opgeslagen. Andere apps tonen hem nadat je daar opnieuw inlogt.",
  pictureRemoved: "Foto verwijderd.",
  accountTitle: "Account",
  accountSub: "Je naam en e-mailadres komen uit je buunk-account.",
  name: "Naam",
  email: "E-mail",
  username: "Gebruikersnaam",
  accountHelp: "Wil je je naam of e-mailadres wijzigen? Vraag het aan wie buunk voor je beheert. Wachtwoord vergeten? Gebruik Wachtwoord vergeten op de inlogpagina.",
  startTitle: "Startpagina voor je browser",
  startSub: "Zoeken, je apps en je snelle links op één lichte pagina. Stel hem in als start- of nieuwe-tabbladpagina van je browser.",
  startAddress: "Adres van de startpagina",
  preview: "Bekijken",
  linksTitle: "Snelle links",
  linksSub: "Op je startpagina, in deze volgorde.",
  linkCount: (n) => (n === 1 ? "1 link" : `${n} links`),
  noLinks: "Nog geen snelle links.",
  addLink: "Snelle link toevoegen",
  linkTitle: "Titel",
  linkAddress: "Adres",
  linkTitlePlaceholder: "Weer",
  linkAddressPlaceholder: "https://buienradar.nl",
  addLinkButton: "Link toevoegen",
  save: "Opslaan",
  cancel: "Annuleren",
  moveUp: (t) => `${t} omhoog`,
  moveDown: (t) => `${t} omlaag`,
  edit: (t) => `${t} bewerken`,
  remove: (t) => `${t} verwijderen`,
  errors: {
    "too-large": "Die foto is groter dan 5 MB.",
    format: "Dat is geen JPEG-, PNG- of WebP-foto.",
    dimensions: "Die foto is te klein of heeft te veel pixels.",
    "invalid-link": "Geef een titel en een adres dat begint met http:// of https://.",
    "too-many": "Je hebt het maximale aantal snelle links (50).",
    other: "Dat lukte niet. Probeer het zo opnieuw.",
  },

  greeting: (hour, name) =>
    `${hour < 6 ? "Goedenavond" : hour < 12 ? "Goedemorgen" : hour < 18 ? "Goedemiddag" : "Goedenavond"}, ${name}`,
  appCount: (n) => (n === 1 ? "1 app op je buunk-account." : `${n} apps op je buunk-account.`),
  noApps: "Nog geen apps op je buunk-account. Vraag het aan wie buunk voor je beheert.",
  yourApps: "Je apps",
  readyNow: (ready, all) => `${ready} van ${all} nu klaar`,
  ready: "Klaar",
  checking: "Controleren…",
  lockedNeedsWarp: "Op slot · WARP nodig",
  public: "Openbaar",
  internalOverWarp: "Intern, via WARP",
  open: "Openen",
  openApp: (name) => `${name} openen`,
  connectWarp: "WARP verbinden",

  warpTitle: "Cloudflare WARP",
  notConnected: "Niet verbonden",
  offWarp: (names) => `${names} openen alleen via WARP. Verbind het en ze gaan hier open.`,
  howToConnect: "Zo verbind je",
  and: "en",

  guideTitle: "Verbinden met WARP",
  guideIntro:
    "Sommige buunk-apps draaien alleen op ons eigen netwerk. WARP verbindt je apparaat daar veilig mee, waar je ook bent. Het kost ongeveer vijf minuten, één keer per apparaat.",
  yourDevice: "Je apparaat",
  platforms: { win: "Windows", mac: "macOS", linux: "Linux", ios: "iPhone en iPad", android: "Android" },
  step1Title: (mobile) => `Installeer de ${mobile ? "Cloudflare One Agent-app" : "Cloudflare One Client"}`,
  step1Text: "De app van Cloudflare, vroeger WARP genoemd. Hij is gratis.",
  download: (mobile, platform) => (mobile ? `Download voor ${platform}` : `Download voor ${platform}`),
  step2Title: "Open hem en kies Zero Trust security",
  step2Text: (mobile) =>
    mobile
      ? "Accepteer het privacybeleid en kies Zero Trust security als de app vraagt waarvoor je hem wilt gebruiken."
      : "Kies op het eerste scherm Zero Trust security. Al eens gebruikt? Open het tandwiel, dan Preferences, Account, Login with Cloudflare Zero Trust.",
  step3Title: "Vul onze teamnaam in",
  teamName: "Teamnaam",
  copy: "Kopiëren",
  copied: "Gekopieerd",
  step4Title: "Log in met je buunk-account",
  step4Text: (mobile) =>
    mobile
      ? "Er opent een inlogpagina. Kies buunk, log in zoals bij de hub, tik dan op Install VPN Profile en sta het toe."
      : "Je browser opent een inlogpagina. Kies buunk, log in zoals bij de hub en kies dan Open Cloudflare WARP om af te ronden.",
  step5Title: "Zet hem aan",
  step5Text: "Zet de schakelaar in de app op Connected en controleer het hier.",
  stillNotConnected: "Nog niet verbonden",
  nowConnected: "Verbonden",
  checkedFromBrowser: "Zojuist gecontroleerd vanuit deze browser.",
  checkAgain: "Opnieuw controleren",
  unlocks: "Wat WARP opent",
  unlocked: "Bereikbaar",
  locked: "Niet bereikbaar",
  noInternalApps: "Geen van je apps heeft nu WARP nodig.",
  faq: [
    ["Moet hij steeds aan blijven?", "Nee. Zet hem aan als je een app met WARP nodig wilt gebruiken, en uit wanneer je wilt."],
    [
      "Ik ben ingelogd maar apps blijven op slot",
      "Kijk of de app Connected zegt en druk op Opnieuw controleren. Nog steeds op slot? Dan heeft je account misschien nog geen toegang tot die app.",
    ],
    ["Wie kan me helpen?", "Vraag het aan wie buunk voor je beheert. Zeg welk apparaat je instelt en bij welke stap je bent."],
  ],
};

const messages = { en, nl } as const;

export type Lang = keyof typeof messages;

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

/** "a", "a and b", "a, b and c" */
export function joinNames(names: string[], and: string): string {
  if (names.length <= 1) return names.join("");
  return names.slice(0, -1).join(", ") + ` ${and} ` + names[names.length - 1];
}
