import { useEffect, useState, type FormEvent } from "react";
import {
  Alert,
  Button,
  ButtonLink,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  DescriptionList,
  Dropzone,
  Field,
  Fieldset,
  Icon,
  Input,
  List,
  ListItem,
  Spinner,
} from "@flatwork-ui/react";
import { ApiError, getJSON, send, type Me, type QuickLink } from "./api";
import type { Messages } from "./i18n";
import { CropDialog } from "./CropDialog";
import { Link } from "./router";
import { Picture } from "./Shell";

// Settings (designs: Settings, SettingsLinks): only the person's own things.

const maxUpload = 5 << 20;

function errorText(m: Messages, e: unknown): string {
  return m.errors[e instanceof ApiError ? e.code : "other"] ?? m.errors.other;
}

export function Settings({ me, m, path, setMe }: { me: Me; m: Messages; path: string; setMe: (me: Me) => void }) {
  const onStart = path === "/settings/start";
  return (
    <>
      <header style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <h1 className="fw-t-display">{m.settings}</h1>
        <p className="fw-t-body fw-t-muted">{m.settingsIntro}</p>
      </header>
      <nav className="fw-tabs" aria-label={m.settingsSections}>
        <Link className={onStart ? "fw-tab" : "fw-tab is-active"} href="/settings" aria-current={onStart ? undefined : "page"}>
          {m.tabProfile}
        </Link>
        <Link className={onStart ? "fw-tab is-active" : "fw-tab"} href="/settings/start" aria-current={onStart ? "page" : undefined}>
          {m.tabStart}
        </Link>
      </nav>
      <div style={{ display: "flex", flexDirection: "column", gap: 16, maxWidth: 760 }}>
        {onStart ? <StartSettings m={m} /> : <Profile me={me} m={m} setMe={setMe} />}
      </div>
    </>
  );
}

function Profile({ me, m, setMe }: { me: Me; m: Messages; setMe: (me: Me) => void }) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ tone: "success" | "error"; text: string }>();

  const [framing, setFraming] = useState<File>();

  // A chosen file is framed first (CropDialog), then uploaded.
  function choose(files: FileList) {
    const file = files[0];
    if (!file) return;
    if (file.size > maxUpload) {
      setMessage({ tone: "error", text: m.errors["too-large"]! });
      return;
    }
    setMessage(undefined);
    setFraming(file);
  }

  async function upload(cropped: Blob) {
    setFraming(undefined);
    const form = new FormData();
    form.append("picture", cropped, "picture.jpg");
    setBusy(true);
    setMessage(undefined);
    try {
      const { picture } = await send<{ picture: string }>("POST", "/api/me/picture", form);
      setMe({ ...me, picture });
      setMessage({ tone: "success", text: m.pictureSaved });
    } catch (e) {
      setMessage({ tone: "error", text: errorText(m, e) });
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    setBusy(true);
    setMessage(undefined);
    try {
      await send("DELETE", "/api/me/picture");
      setMe({ ...me, picture: "" });
      setMessage({ tone: "success", text: m.pictureRemoved });
    } catch (e) {
      setMessage({ tone: "error", text: errorText(m, e) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Card aria-labelledby="pic-title">
        <CardHeader>
          <div>
            <h2 className="fw-card-title" id="pic-title">
              {m.pictureTitle}
            </h2>
            <p className="fw-card-sub">{m.pictureSub}</p>
          </div>
        </CardHeader>
        <CardBody style={{ gap: 16 }}>
          <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 20 }}>
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 10 }}>
              <span role="img" aria-label={m.currentPicture}>
                <Picture me={me} size={96} />
              </span>
              <Button variant="ghost" size="sm" disabled={!me.picture || busy} onClick={remove}>
                {m.removePicture}
              </Button>
            </div>
            {/* Dropzone puts its props on the hidden file input, so the
                layout goes on a box around it. */}
            <div style={{ flex: "1 1 320px" }}>
              <Dropzone
                accept="image/jpeg,image/png,image/webp"
                disabled={busy}
                onFiles={choose}
                title={
                  busy ? (
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
                      <Spinner /> {m.uploading}
                    </span>
                  ) : (
                    <>
                      <strong>{m.dropTitle}</strong> {m.dropOr}
                    </>
                  )
                }
                hint={m.dropHint}
              />
            </div>
          </div>
          {message && <Alert tone={message.tone}>{message.text}</Alert>}
          {framing && <CropDialog file={framing} m={m} onCancel={() => setFraming(undefined)} onDone={upload} />}
        </CardBody>
      </Card>

      <Card aria-labelledby="acct-title">
        <CardHeader>
          <div>
            <h2 className="fw-card-title" id="acct-title">
              {m.accountTitle}
            </h2>
            <p className="fw-card-sub">{m.accountSub}</p>
          </div>
        </CardHeader>
        <CardBody>
          <DescriptionList
            items={[
              { term: m.name, value: me.name || "—" },
              { term: m.email, value: me.email || "—" },
              { term: m.username, value: me.username || "—", mono: true },
            ]}
          />
        </CardBody>
        <CardFooter style={{ justifyContent: "flex-start" }}>
          <p className="fw-t-caption">{m.accountHelp}</p>
        </CardFooter>
      </Card>
    </>
  );
}

function StartSettings({ m }: { m: Messages }) {
  const [links, setLinks] = useState<QuickLink[]>();
  const [error, setError] = useState<string>();
  const [copied, setCopied] = useState(false);
  const [editing, setEditing] = useState<number>();
  const startURL = `${location.origin}/start`;

  useEffect(() => {
    getJSON<QuickLink[]>("/api/links").then(setLinks, (e) => setError(errorText(m, e)));
  }, [m]);

  async function run(action: () => Promise<void>) {
    setError(undefined);
    try {
      await action();
    } catch (e) {
      setError(errorText(m, e));
    }
  }

  const move = (i: number, by: number) =>
    run(async () => {
      const next = [...links!];
      [next[i], next[i + by]] = [next[i + by]!, next[i]!];
      setLinks(next);
      await send("PUT", "/api/links/order", { ids: next.map((l) => l.id) });
    });

  return (
    <>
      <Card aria-labelledby="sp-title">
        <CardHeader>
          <div>
            <h2 className="fw-card-title" id="sp-title">
              {m.startTitle}
            </h2>
            <p className="fw-card-sub">{m.startSub}</p>
          </div>
        </CardHeader>
        <CardBody>
          <Field label={m.startAddress} id="sp-url">
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Input id="sp-url" mono readOnly value={startURL} style={{ flex: "1 1 260px" }} />
              <Button
                leadingIcon={<Icon name={copied ? "check" : "content_copy"} />}
                onClick={() =>
                  navigator.clipboard?.writeText(startURL).then(() => {
                    setCopied(true);
                    setTimeout(() => setCopied(false), 2000);
                  })
                }
              >
                <span aria-live="polite">{copied ? m.copied : m.copy}</span>
              </Button>
              <ButtonLink variant="ghost" href="/start" trailingIcon={<Icon name="arrow_outward" />}>
                {m.preview}
              </ButtonLink>
            </div>
          </Field>
        </CardBody>
      </Card>

      <Card aria-labelledby="ql-title">
        <CardHeader action={links && <span className="fw-t-caption">{m.linkCount(links.length)}</span>}>
          <div>
            <h2 className="fw-card-title" id="ql-title">
              {m.linksTitle}
            </h2>
            <p className="fw-card-sub">{m.linksSub}</p>
          </div>
        </CardHeader>
        <CardBody style={{ gap: 16 }}>
          {!links && !error && <Spinner label={m.loading} />}
          {links && links.length === 0 && <p className="fw-t-body fw-t-muted">{m.noLinks}</p>}
          {links && links.length > 0 && (
            <List>
              {links.map((l, i) =>
                editing === l.id ? (
                  <li key={l.id}>
                    <LinkForm
                      m={m}
                      initial={l}
                      submitLabel={m.save}
                      onCancel={() => setEditing(undefined)}
                      onSubmit={(title, url) =>
                        run(async () => {
                          const saved = await send<QuickLink>("PUT", `/api/links/${l.id}`, { title, url });
                          setLinks(links.map((x) => (x.id === l.id ? saved : x)));
                          setEditing(undefined);
                        })
                      }
                    />
                  </li>
                ) : (
                  <ListItem
                    key={l.id}
                    title={l.title}
                    meta={<span className="fw-t-mono">{l.url}</span>}
                    trailing={
                      <span style={{ display: "flex", gap: 2 }}>
                        <Button variant="ghost" size="sm" iconOnly aria-label={m.moveUp(l.title)} disabled={i === 0} onClick={() => move(i, -1)}>
                          <Icon name="arrow_upward" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          iconOnly
                          aria-label={m.moveDown(l.title)}
                          disabled={i === links.length - 1}
                          onClick={() => move(i, 1)}
                        >
                          <Icon name="arrow_downward" />
                        </Button>
                        <Button variant="ghost" size="sm" iconOnly aria-label={m.edit(l.title)} onClick={() => setEditing(l.id)}>
                          <Icon name="edit" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          iconOnly
                          aria-label={m.remove(l.title)}
                          onClick={() =>
                            run(async () => {
                              await send("DELETE", `/api/links/${l.id}`);
                              setLinks(links.filter((x) => x.id !== l.id));
                            })
                          }
                        >
                          <Icon name="delete" />
                        </Button>
                      </span>
                    }
                  />
                ),
              )}
            </List>
          )}
          <Fieldset legend={m.addLink}>
            <LinkForm
              m={m}
              submitLabel={m.addLinkButton}
              resetOnSubmit
              onSubmit={(title, url) =>
                run(async () => {
                  const added = await send<QuickLink>("POST", "/api/links", { title, url });
                  setLinks([...(links ?? []), added]);
                })
              }
            />
          </Fieldset>
          {error && <Alert tone="error">{error}</Alert>}
        </CardBody>
      </Card>
    </>
  );
}

function LinkForm({
  m,
  initial,
  submitLabel,
  resetOnSubmit,
  onSubmit,
  onCancel,
}: {
  m: Messages;
  initial?: QuickLink;
  submitLabel: string;
  resetOnSubmit?: boolean;
  onSubmit: (title: string, url: string) => Promise<void>;
  onCancel?: () => void;
}) {
  const [title, setTitle] = useState(initial?.title ?? "");
  const [url, setUrl] = useState(initial?.url ?? "");
  const [busy, setBusy] = useState(false);
  const idBase = initial ? `edit-${initial.id}` : "add";

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    await onSubmit(title, url);
    setBusy(false);
    if (resetOnSubmit) {
      setTitle("");
      setUrl("");
    }
  }

  return (
    <form onSubmit={submit} style={{ display: "flex", flexWrap: "wrap", gap: 12, alignItems: "flex-end", width: "100%" }}>
      <Field label={m.linkTitle} id={`${idBase}-title`} style={{ flex: "1 1 180px" }}>
        <Input
          id={`${idBase}-title`}
          required
          maxLength={80}
          value={title}
          placeholder={m.linkTitlePlaceholder}
          onChange={(e) => setTitle(e.target.value)}
        />
      </Field>
      <Field label={m.linkAddress} id={`${idBase}-url`} style={{ flex: "2 1 260px" }}>
        <Input
          id={`${idBase}-url`}
          mono
          required
          value={url}
          placeholder={m.linkAddressPlaceholder}
          inputMode="url"
          onChange={(e) => setUrl(e.target.value)}
        />
      </Field>
      <div style={{ display: "flex", gap: 8 }}>
        {onCancel && (
          <Button variant="ghost" onClick={onCancel}>
            {m.cancel}
          </Button>
        )}
        <Button type="submit" loading={busy} leadingIcon={initial ? undefined : <Icon name="add" />}>
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}
