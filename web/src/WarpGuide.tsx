import { useState, type ReactNode } from "react";
import {
  Accordion,
  AccordionItem,
  Badge,
  Breadcrumb,
  Button,
  ButtonLink,
  Card,
  CardBody,
  CardHeader,
  Icon,
  Input,
  SegmentedControl,
  Spinner,
  type IconName,
} from "@flatwork-ui/react";
import type { App } from "./api";
import { cloudflareOrange } from "./cloudflare";
import type { Messages } from "./i18n";
import { Link } from "./router";
import type { Reach } from "./warp";

// How to connect WARP (design: WarpGuide.dc.html), with a live check of the
// person's internal apps at the end.

// The Zero Trust team people enrol in (hgd1-k8s docs/concepts/internal-access.md).
const team = "buunk";
const downloadURL = "https://developers.cloudflare.com/cloudflare-one/team-and-resources/devices/cloudflare-one-client/download/";

type Platform = keyof Messages["platforms"];
const platforms: { id: Platform; icon: IconName; mobile: boolean }[] = [
  { id: "win", icon: "desktop_windows", mobile: false },
  { id: "mac", icon: "laptop_mac", mobile: false },
  { id: "linux", icon: "terminal", mobile: false },
  { id: "ios", icon: "ios", mobile: true },
  { id: "android", icon: "android", mobile: true },
];

function guessPlatform(): Platform {
  const ua = navigator.userAgent;
  if (/iPhone|iPad|iPod/.test(ua)) return "ios";
  if (/Android/.test(ua)) return "android";
  if (/Mac/.test(ua)) return "mac";
  if (/Linux/.test(ua)) return "linux";
  return "win";
}

export function WarpGuide({
  m,
  apps,
  reach,
  recheck,
}: {
  m: Messages;
  apps: App[];
  reach: Record<string, Reach>;
  recheck: () => void;
}) {
  const [platform, setPlatform] = useState<Platform>(guessPlatform);
  const [copied, setCopied] = useState(false);
  const cur = platforms.find((p) => p.id === platform)!;
  const internal = apps.filter((a) => a.internal);
  const checking = internal.some((a) => (reach[a.id] ?? "checking") === "checking");
  const onWarp = internal.some((a) => reach[a.id] === "ok");

  return (
    <>
      <header style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        <Breadcrumb
          items={[{ label: m.home, href: "/" }, { label: m.guideTitle }]}
          renderLink={(c) => <Link href={c.href!}>{c.label}</Link>}
        />
        <h1 className="fw-t-display">{m.guideTitle}</h1>
        <p className="fw-t-body fw-t-muted" style={{ maxWidth: 620 }}>
          {m.guideIntro}
        </p>
      </header>

      <div style={{ display: "flex", flexWrap: "wrap", gap: 24, alignItems: "flex-start" }}>
        <div style={{ flex: "999 1 520px", minWidth: 0, display: "flex", flexDirection: "column", gap: 20, maxWidth: 760 }}>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <span className="fw-t-label" id="platform-label">
              {m.yourDevice}
            </span>
            <SegmentedControl
              label={m.yourDevice}
              aria-labelledby="platform-label"
              value={platform}
              onValueChange={(v) => setPlatform(v as Platform)}
              options={platforms.map((p) => ({ value: p.id, label: m.platforms[p.id], icon: <Icon name={p.icon} /> }))}
              style={{ flexWrap: "wrap", alignSelf: "flex-start" }}
            />
          </div>

          <ol style={{ listStyle: "none", margin: 0, padding: 0, display: "flex", flexDirection: "column" }}>
            <Step n={1} title={m.step1Title(cur.mobile)}>
              <p className="fw-t-body fw-t-muted">{m.step1Text}</p>
              <ButtonLink
                href={downloadURL}
                target="_blank"
                rel="noreferrer"
                leadingIcon={<Icon name="download" />}
                // Cloudflare's app, in its orange; dark text keeps 6.6:1.
                style={{ alignSelf: "flex-start", background: cloudflareOrange, borderColor: cloudflareOrange, color: "#1b1818" }}
              >
                {m.download(cur.mobile, m.platforms[cur.id])}
              </ButtonLink>
            </Step>
            <Step n={2} title={m.step2Title}>
              <p className="fw-t-body fw-t-muted">{m.step2Text(cur.mobile)}</p>
            </Step>
            <Step n={3} title={m.step3Title}>
              <div className="fw-field" style={{ maxWidth: 420 }}>
                <label className="fw-label" htmlFor="team">
                  {m.teamName}
                </label>
                <div style={{ display: "flex", gap: 8 }}>
                  <Input id="team" mono readOnly value={team} style={{ flex: 1 }} />
                  <Button
                    leadingIcon={<Icon name={copied ? "check" : "content_copy"} />}
                    onClick={() =>
                      navigator.clipboard?.writeText(team).then(() => {
                        setCopied(true);
                        setTimeout(() => setCopied(false), 2000);
                      })
                    }
                  >
                    <span aria-live="polite">{copied ? m.copied : m.copy}</span>
                  </Button>
                </div>
              </div>
            </Step>
            <Step n={4} title={m.step4Title}>
              <p className="fw-t-body fw-t-muted">{m.step4Text(cur.mobile)}</p>
            </Step>
            <Step n={5} title={m.step5Title} last>
              <p className="fw-t-body fw-t-muted">{m.step5Text}</p>
              {internal.length > 0 && (
                <Card style={{ flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 12, padding: "14px 16px" }}>
                  <span aria-live="polite">
                    {checking ? (
                      <Badge>
                        <Spinner /> {m.checking}
                      </Badge>
                    ) : onWarp ? (
                      <Badge tone="success" dot>
                        {m.nowConnected}
                      </Badge>
                    ) : (
                      <Badge style={{ borderColor: cloudflareOrange }}>
                        <span className="fw-dot" style={{ background: cloudflareOrange }} />
                        {m.stillNotConnected}
                      </Badge>
                    )}
                  </span>
                  <span className="fw-t-caption" style={{ flex: "1 1 200px" }}>
                    {m.checkedFromBrowser}
                  </span>
                  <Button size="sm" leadingIcon={<Icon name="refresh" />} onClick={recheck} disabled={checking}>
                    {m.checkAgain}
                  </Button>
                </Card>
              )}
            </Step>
          </ol>
        </div>

        <aside style={{ flex: "1 1 280px", minWidth: 0, display: "flex", flexDirection: "column", gap: 16 }}>
          <Card aria-labelledby="unlocks-title">
            <CardHeader>
              <h2 className="fw-card-title" id="unlocks-title">
                {m.unlocks}
              </h2>
            </CardHeader>
            <CardBody style={{ gap: 10 }}>
              {internal.length === 0 && <p className="fw-t-body fw-t-muted">{m.noInternalApps}</p>}
              {internal.map((a) => (
                <div key={a.id} style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <span
                    aria-hidden="true"
                    style={{
                      display: "grid",
                      placeContent: "center",
                      width: 32,
                      height: 32,
                      borderRadius: "var(--fw-r-sm)",
                      background: "var(--fw-surface-2)",
                      border: "1px solid var(--fw-border)",
                      color: "var(--fw-heading)",
                    }}
                  >
                    <Icon name={a.icon as IconName} />
                  </span>
                  <span className="fw-t-label" style={{ flex: 1 }}>
                    {a.name}
                  </span>
                  {reach[a.id] === "ok" ? (
                    <Icon name="lock_open" label={m.unlocked} style={{ color: "var(--fw-success)" }} />
                  ) : (
                    <Icon name="lock" label={m.locked} style={{ color: "var(--fw-muted)" }} />
                  )}
                </div>
              ))}
            </CardBody>
          </Card>
          <Accordion>
            {m.faq.map(([q, a]) => (
              <AccordionItem key={q} title={q}>
                {a}
              </AccordionItem>
            ))}
          </Accordion>
        </aside>
      </div>
    </>
  );
}

function Step({ n, title, last, children }: { n: number; title: string; last?: boolean; children: ReactNode }) {
  return (
    <li style={{ display: "flex", gap: 16 }}>
      <div style={{ display: "flex", flexDirection: "column", alignItems: "center" }} aria-hidden="true">
        <span className="fw-step-n" style={{ flex: "none", color: "var(--fw-heading)" }}>
          {n}
        </span>
        {!last && <span style={{ flex: 1, width: 1, background: "var(--fw-border-strong)", margin: "6px 0" }} />}
      </div>
      <div
        style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column", gap: 10, padding: last ? "2px 0 0" : "2px 0 28px" }}
      >
        <h2 className="fw-t-heading">{title}</h2>
        {children}
      </div>
    </li>
  );
}
