import { useState, type FormEvent } from "react";
import {
  Affix,
  Alert,
  Breadcrumb,
  Button,
  ButtonLink,
  Dialog,
  DropdownMenu,
  EmptyState,
  Field,
  Icon,
  Input,
  InputGroup,
  MenuItem,
  MenuSeparator,
} from "@flatwork-ui/react";
import { ApiError, send, type HubPage } from "./api";
import type { Messages } from "./i18n";
import { navigate } from "./router";

// My pages (designs: NewPage, Page, CantEmbed): a person's own pages, each
// showing one site. Only theirs; the server answers for no one else.

function errorText(m: Messages, e: unknown): string {
  return m.errors[e instanceof ApiError ? e.code : "other"] ?? m.errors.other;
}

export function host(url: string): string {
  try {
    const u = new URL(url);
    return u.host + (u.pathname === "/" ? "" : u.pathname);
  } catch {
    return url;
  }
}

/** The address without https://, for the field that has it as an affix. */
const bare = (url: string) => url.replace(/^https:\/\//i, "");

type Mode = "new" | "rename" | "address";

/** New page, rename, or change address: one form, the fields it needs. */
export function PageDialog({
  m,
  mode,
  page,
  onClose,
  onSaved,
}: {
  m: Messages;
  mode: Mode;
  page?: HubPage;
  onClose: () => void;
  onSaved: (p: HubPage) => void;
}) {
  const [title, setTitle] = useState(page?.title ?? "");
  const [url, setUrl] = useState(page ? bare(page.url) : "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    try {
      const body = { title, url: url.includes("://") ? url : "https://" + url };
      const saved = page
        ? await send<HubPage>("PUT", `/api/pages/${page.id}`, body)
        : await send<HubPage>("POST", "/api/pages", body);
      onSaved(saved);
    } catch (err) {
      setError(errorText(m, err));
      setBusy(false);
    }
  }

  const heading = mode === "new" ? m.newPage : mode === "rename" ? m.renamePage : m.changeAddress;
  const sub = mode === "new" ? m.newPageSub : mode === "address" ? m.changeAddressSub : undefined;
  // A new address is checked on the server, which can take a few seconds.
  const checking = busy && mode !== "rename";

  return (
    <Dialog
      open
      onClose={onClose}
      title={heading}
      subtitle={sub}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            {m.cancel}
          </Button>
          <Button type="submit" form="page-form" variant="primary" loading={busy}>
            {checking ? m.checkingSite : mode === "new" ? m.createPage : m.save}
          </Button>
        </>
      }
    >
      <form id="page-form" onSubmit={submit} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        {mode !== "address" && (
          <Field label={m.pageTitle} id="page-title">
            <Input
              id="page-title"
              required
              maxLength={80}
              autoFocus
              value={title}
              placeholder={m.pageTitlePlaceholder}
              onChange={(e) => setTitle(e.target.value)}
            />
          </Field>
        )}
        {mode !== "rename" && (
          <Field label={m.pageAddress} id="page-url" help={m.pageAddressHelp}>
            <InputGroup>
              <Affix boxed>https://</Affix>
              <Input
                id="page-url"
                mono
                required
                autoFocus={mode === "address"}
                inputMode="url"
                value={url}
                placeholder={m.pageAddressPlaceholder}
                // Pasting a whole address works too: the field has https:// already.
                onChange={(e) => setUrl(bare(e.target.value))}
              />
            </InputGroup>
          </Field>
        )}
        {error && <Alert tone="error">{error}</Alert>}
      </form>
    </Dialog>
  );
}

/** One page: the site in a sandboxed frame, or a card that opens it. */
export function PageView({
  m,
  pages,
  id,
  setPages,
}: {
  m: Messages;
  pages: HubPage[];
  id: number;
  setPages: (p: HubPage[]) => void;
}) {
  const [dialog, setDialog] = useState<Mode | "delete">();
  const [reloads, setReloads] = useState(0);
  const [error, setError] = useState<string>();
  const i = pages.findIndex((p) => p.id === id);
  const page = pages[i];

  if (!page) {
    return (
      <EmptyState
        icon={<Icon name="link_off" />}
        title={m.pageNotFound}
        action={<Button onClick={() => navigate("/")}>{m.backHome}</Button>}
      />
    );
  }

  async function run(action: () => Promise<void>) {
    setError(undefined);
    try {
      await action();
    } catch (e) {
      setError(errorText(m, e));
    }
  }

  const move = (by: number) =>
    run(async () => {
      const next = [...pages];
      [next[i], next[i + by]] = [next[i + by]!, next[i]!];
      setPages(next);
      await send("PUT", "/api/pages/order", { ids: next.map((p) => p.id) });
    });

  const remove = () =>
    run(async () => {
      await send("DELETE", `/api/pages/${page.id}`);
      setDialog(undefined);
      setPages(pages.filter((p) => p.id !== page.id));
      navigate("/");
    });

  const where = host(page.url);
  const siteHost = new URL(page.url).host;

  return (
    <div className="hub-page">
      <header style={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
        <div style={{ display: "flex", flexDirection: "column", gap: 4, minWidth: 0 }}>
          <Breadcrumb items={[{ label: m.myPages }, { label: page.title }]} />
          <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap", minWidth: 0 }}>
            <h1 className="fw-t-title">{page.title}</h1>
            <span className="fw-t-mono fw-t-muted hub-page-host">{where}</span>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
          {!page.refused && (
            <>
              <Button variant="ghost" size="sm" iconOnly aria-label={m.reloadPage} title={m.reloadPage} onClick={() => setReloads((n) => n + 1)}>
                <Icon name="refresh" />
              </Button>
              <ButtonLink
                variant="ghost"
                size="sm"
                iconOnly
                href={page.url}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={m.openInNewTab}
                title={m.openInNewTab}
              >
                <Icon name="open_in_new" />
              </ButtonLink>
            </>
          )}
          <DropdownMenu
            alignEnd
            label={m.pageOptions}
            trigger={
              <Button variant="secondary" size="sm" iconOnly aria-label={m.pageOptions} title={m.pageOptions}>
                <Icon name="more_horiz" />
              </Button>
            }
          >
            <MenuItem icon={<Icon name="edit" />} onClick={() => setDialog("rename")}>
              {m.rename}
            </MenuItem>
            <MenuItem icon={<Icon name="link" />} onClick={() => setDialog("address")}>
              {m.changeAddress}
            </MenuItem>
            <MenuItem icon={<Icon name="arrow_upward" />} disabled={i === 0} onClick={() => move(-1)}>
              {m.moveUpPage}
            </MenuItem>
            <MenuItem icon={<Icon name="arrow_downward" />} disabled={i === pages.length - 1} onClick={() => move(1)}>
              {m.moveDownPage}
            </MenuItem>
            <MenuSeparator />
            <MenuItem icon={<Icon name="delete" />} danger onClick={() => setDialog("delete")}>
              {m.deletePage}
            </MenuItem>
          </DropdownMenu>
        </div>
      </header>

      {error && <Alert tone="error">{error}</Alert>}

      {page.refused ? (
        <section aria-labelledby="noembed-title" className="hub-frame" style={{ display: "grid", placeItems: "center", padding: 40 }}>
          <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 12, maxWidth: 420, textAlign: "center" }}>
            <span className="fw-empty-icon">
              <Icon name="link_off" />
            </span>
            <h2 className="fw-t-heading" id="noembed-title">
              {m.cantEmbedTitle(page.title)}
            </h2>
            <p className="fw-t-body fw-t-muted">{m.cantEmbedText(siteHost)}</p>
            <ButtonLink variant="primary" href={page.url} target="_blank" rel="noopener noreferrer" leadingIcon={<Icon name="open_in_new" />}>
              {m.openSite(siteHost)}
            </ButtonLink>
            <span className="fw-t-caption">
              {m.checkedWhenSaved} <code>{page.refused}</code>
            </span>
          </div>
        </section>
      ) : (
        <section aria-label={m.embedded(siteHost)} className="hub-frame">
          {/* Another origin, sandboxed: it may run its own scripts and forms
              and open popups, but not navigate the hub. The key reloads it. */}
          <iframe
            key={`${page.url}#${reloads}`}
            src={page.url}
            title={page.title}
            sandbox="allow-scripts allow-forms allow-popups allow-same-origin"
            referrerPolicy="strict-origin-when-cross-origin"
          />
        </section>
      )}

      {(dialog === "rename" || dialog === "address") && (
        <PageDialog
          m={m}
          mode={dialog}
          page={page}
          onClose={() => setDialog(undefined)}
          onSaved={(saved) => {
            setDialog(undefined);
            setPages(pages.map((p) => (p.id === saved.id ? saved : p)));
          }}
        />
      )}
      {dialog === "delete" && (
        <Dialog
          open
          alert
          onClose={() => setDialog(undefined)}
          title={m.deletePageTitle(page.title)}
          footer={
            <>
              <Button variant="secondary" onClick={() => setDialog(undefined)}>
                {m.cancel}
              </Button>
              <Button variant="danger" onClick={remove}>
                {m.delete}
              </Button>
            </>
          }
        >
          <p className="fw-t-body">{m.deletePageText}</p>
        </Dialog>
      )}
    </div>
  );
}
