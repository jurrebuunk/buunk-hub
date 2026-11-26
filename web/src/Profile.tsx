import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  DescriptionList,
  Dialog,
  Field,
  Icon,
  Input,
  InputGroup,
  Select,
  Spinner,
} from "@flatwork-ui/react";
import { getJSON, send, type Me, type Profile } from "./api";
import { CropDialog } from "./CropDialog";
import { pickLang, type Messages } from "./i18n";
import { Picture } from "./Shell";
import { errorText, usePicture } from "./usePicture";

// The person's account: name, email and language, in the first-visit
// dialog (design: Welcome) and in Settings. Name and email are saved in
// Keycloak, so every buunk app gets them; the language is the hub's own.

type Form = Pick<Profile, "firstName" | "lastName" | "email" | "lang">;

function useProfile(m: Messages) {
  const [profile, setProfile] = useState<Profile>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    getJSON<Profile>("/api/me/profile").then(setProfile, (e) => setError(errorText(m, e)));
  }, [m]);
  return { profile, setProfile, error };
}

/** Saves the form; answers the new Me and what to tell the person. */
async function save(me: Me, m: Messages, form: Form): Promise<{ me: Me; note: string }> {
  const res = await send<{ name: string; email: string; emailChanged: boolean; mailFailed: boolean }>("PUT", "/api/me/profile", form);
  const lang = form.lang || pickLang(navigator.languages);
  return {
    me: { ...me, name: res.name, email: res.email, lang, welcomed: true },
    note: res.mailFailed ? m.verifyEmailFailed : res.emailChanged ? m.verifyEmailSent(res.email) : m.profileSaved,
  };
}

function ProfileFields({
  m,
  idBase,
  form,
  setForm,
  verified,
}: {
  m: Messages;
  idBase: string;
  form: Form;
  setForm: (f: Form) => void;
  verified: boolean;
}) {
  return (
    <div className="hub-profile-grid">
      <Field label={m.firstName} id={`${idBase}-first`}>
        <Input
          id={`${idBase}-first`}
          required
          maxLength={255}
          autoComplete="given-name"
          value={form.firstName}
          onChange={(e) => setForm({ ...form, firstName: e.target.value })}
        />
      </Field>
      <Field label={m.lastName} id={`${idBase}-last`}>
        <Input
          id={`${idBase}-last`}
          required
          maxLength={255}
          autoComplete="family-name"
          value={form.lastName}
          onChange={(e) => setForm({ ...form, lastName: e.target.value })}
        />
      </Field>
      <Field label={m.email} id={`${idBase}-email`} help={m.emailHelp} className="hub-span">
        <InputGroup>
          <Input
            id={`${idBase}-email`}
            type="email"
            required
            maxLength={255}
            autoComplete="email"
            value={form.email}
            onChange={(e) => setForm({ ...form, email: e.target.value })}
          />
          <span className="fw-affix">
            <Badge tone={verified ? "success" : "warning"}>
              <Icon name={verified ? "verified" : "mark_email_unread"} size={13} />
              {verified ? m.verified : m.notVerified}
            </Badge>
          </span>
        </InputGroup>
      </Field>
      <Field label={m.language} id={`${idBase}-lang`} help={m.languageHelp} className="hub-span">
        <Select id={`${idBase}-lang`} value={form.lang} onChange={(e) => setForm({ ...form, lang: e.target.value })}>
          <option value="">{m.languageAuto}</option>
          <option value="nl" lang="nl">
            Nederlands
          </option>
          <option value="en" lang="en">
            English
          </option>
        </Select>
      </Field>
    </div>
  );
}

const formOf = (p: Profile): Form => ({ firstName: p.firstName, lastName: p.lastName, email: p.email, lang: p.lang });

/** The first visit: picture, name, email and language in one go. */
export function WelcomeDialog({ me, m, setMe, onClose }: { me: Me; m: Messages; setMe: (me: Me) => void; onClose: () => void }) {
  const { profile, error: loadError } = useProfile(m);
  const [form, setForm] = useState<Form>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const pic = usePicture(me, m, setMe);
  const fileInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (profile) setForm(formOf(profile));
  }, [profile]);

  async function skip() {
    onClose();
    setMe({ ...me, welcomed: true });
    // Not asked again; failing that, it just shows next time.
    send("POST", "/api/me/welcomed").catch(() => {});
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setError(undefined);
    try {
      const { me: next } = await save(me, m, form);
      setMe(next);
      onClose();
    } catch (err) {
      setError(errorText(m, err));
      setBusy(false);
    }
  }

  const first = profile?.firstName || me.name.split(" ")[0] || me.name;

  return (
    <>
      <Dialog
        open={!pic.framing}
        onClose={skip}
        title={m.welcomeTitle(first)}
        subtitle={m.welcomeSub}
        className="hub-welcome"
        footer={
          <div style={{ display: "flex", justifyContent: "space-between", width: "100%", gap: 8 }}>
            <Button variant="ghost" onClick={skip}>
              {m.skipForNow}
            </Button>
            <Button type="submit" form="welcome-form" variant="primary" loading={busy} disabled={!form}>
              {m.saveProfile}
            </Button>
          </div>
        }
      >
        <form id="welcome-form" onSubmit={submit} style={{ display: "flex", flexDirection: "column", gap: 20 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 20, flexWrap: "wrap" }}>
            <label className="hub-pick-picture">
              {me.picture ? <Picture me={me} size={112} /> : <Icon name="add_a_photo" size={32} />}
              <input
                ref={fileInput}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                aria-label={m.choosePictureLabel}
                className="hub-visually-hidden"
                onChange={(e) => {
                  pic.choose(e.target.files);
                  e.target.value = "";
                }}
              />
            </label>
            <div style={{ flex: "1 1 240px", display: "flex", flexDirection: "column", gap: 8 }}>
              <span className="fw-t-label">{m.pictureTitle}</span>
              <span className="fw-t-caption">{m.welcomePictureHelp}</span>
              <Button
                variant="secondary"
                size="sm"
                loading={pic.busy}
                leadingIcon={<Icon name="upload" />}
                style={{ alignSelf: "flex-start" }}
                onClick={() => fileInput.current?.click()}
              >
                {pic.busy ? m.uploading : m.choosePicture}
              </Button>
            </div>
          </div>
          {pic.message && <Alert tone={pic.message.tone}>{pic.message.text}</Alert>}
          <hr className="hub-rule" />
          {form && profile ? (
            <ProfileFields m={m} idBase="wl" form={form} setForm={setForm} verified={profile.emailVerified} />
          ) : loadError ? (
            <Alert tone="error">{loadError}</Alert>
          ) : (
            <Spinner label={m.loading} />
          )}
          {error ? <Alert tone="error">{error}</Alert> : <Alert tone="neutral">{m.savedToAccount}</Alert>}
        </form>
      </Dialog>
      {pic.framing && <CropDialog file={pic.framing} m={m} onCancel={pic.cancelFraming} onDone={pic.upload} />}
    </>
  );
}

/** Settings → Account: the same fields, and the groups the person is in. */
export function AccountCard({ me, m, setMe }: { me: Me; m: Messages; setMe: (me: Me) => void }) {
  const { profile, setProfile, error: loadError } = useProfile(m);
  const [form, setForm] = useState<Form>();
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ tone: "success" | "error"; text: string }>();

  useEffect(() => {
    if (profile) setForm(formOf(profile));
  }, [profile]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!form || !profile) return;
    setBusy(true);
    setMessage(undefined);
    try {
      const { me: next, note } = await save(me, m, form);
      setMe(next);
      const emailChanged = form.email.trim().toLowerCase() !== profile.email.toLowerCase();
      setProfile({ ...profile, ...form, emailVerified: profile.emailVerified && !emailChanged });
      setMessage({ tone: "success", text: note });
    } catch (err) {
      setMessage({ tone: "error", text: errorText(m, err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card aria-labelledby="acct-title">
      <CardHeader>
        <div>
          <h2 className="fw-card-title" id="acct-title">
            {m.accountTitle}
          </h2>
          <p className="fw-card-sub">{m.accountSub}</p>
        </div>
      </CardHeader>
      <form onSubmit={submit}>
        <CardBody style={{ gap: 16 }}>
          {form && profile ? (
            <>
              <ProfileFields m={m} idBase="acct" form={form} setForm={setForm} verified={profile.emailVerified} />
              <DescriptionList
                items={[
                  { term: m.username, value: me.username || "—", mono: true },
                  {
                    term: m.groups,
                    value: profile.groups.length ? (
                      <span style={{ display: "inline-flex", flexWrap: "wrap", gap: 6 }}>
                        {profile.groups.map((g) => (
                          <Badge key={g}>{g}</Badge>
                        ))}
                      </span>
                    ) : (
                      m.noGroups
                    ),
                  },
                ]}
              />
            </>
          ) : loadError ? (
            <Alert tone="error">{loadError}</Alert>
          ) : (
            <Spinner label={m.loading} />
          )}
          {message && <Alert tone={message.tone}>{message.text}</Alert>}
        </CardBody>
        <CardFooter style={{ justifyContent: "space-between", flexWrap: "wrap", gap: 12 }}>
          <p className="fw-t-caption">{m.accountHelp}</p>
          <Button type="submit" variant="primary" size="sm" loading={busy} disabled={!form}>
            {m.saveProfile}
          </Button>
        </CardFooter>
      </form>
    </Card>
  );
}
