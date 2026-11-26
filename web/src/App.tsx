import { useEffect } from "react";
import { EmptyState, FlatworkRoot, Icon } from "@flatwork-ui/react";
import { pickLang, t } from "./i18n";

const lang = pickLang(navigator.languages);
const m = t(lang);

export function App() {
  useEffect(() => {
    document.documentElement.lang = lang;
    document.title = m.title;
  }, []);

  return (
    <FlatworkRoot mode="system" style={{ minHeight: "100vh", display: "grid", placeItems: "center" }}>
      <main>
        <h1 className="fw-t-title" style={{ textAlign: "center" }}>{m.title}</h1>
        <EmptyState icon={<Icon name="construction" />} title={m.comingSoon} description={m.comingSoonText} />
      </main>
    </FlatworkRoot>
  );
}
