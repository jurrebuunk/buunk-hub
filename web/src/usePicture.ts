import { useState } from "react";
import { ApiError, send, type Me } from "./api";
import type { Messages } from "./i18n";

// Choosing, framing and uploading a profile picture, shared by Settings and
// the first-visit dialog: a chosen file is framed first (CropDialog), then
// uploaded.

const maxUpload = 5 << 20;

export function errorText(m: Messages, e: unknown): string {
  return m.errors[e instanceof ApiError ? e.code : "other"] ?? m.errors.other;
}

export function usePicture(me: Me, m: Messages, setMe: (me: Me) => void) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ tone: "success" | "error"; text: string }>();
  const [framing, setFraming] = useState<File>();

  function choose(files: FileList | null) {
    const file = files?.[0];
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

  return { busy, message, framing, choose, upload, remove, cancelFraming: () => setFraming(undefined) };
}
