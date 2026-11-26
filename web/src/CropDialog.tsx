import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";
import { Alert, Button, Dialog, Range } from "@flatwork-ui/react";
import type { Messages } from "./i18n";

// Framing a new profile picture before it is uploaded: drag to move, slider
// (or wheel, or + and -) to zoom, arrow keys to move. The browser cuts out
// the square; it also applies a phone photo's rotation, which the server
// would not. The server re-encodes whatever it gets.

const view = 288; // the square on screen, px
const outSize = 1024; // what is uploaded (the server makes 512 of it)
const maxZoom = 4;

interface Frame {
  zoom: number; // 1 = the picture just covers the square
  x: number; // top-left of the picture, relative to the square, px
  y: number;
}

export function CropDialog({
  file,
  m,
  onCancel,
  onDone,
}: {
  file: File;
  m: Messages;
  onCancel: () => void;
  onDone: (cropped: Blob) => void;
}) {
  const [img, setImg] = useState<HTMLImageElement>();
  const [failed, setFailed] = useState(false);
  const [frame, setFrame] = useState<Frame>({ zoom: 1, x: 0, y: 0 });
  const drag = useRef<{ px: number; py: number; x: number; y: number } | undefined>(undefined);

  useEffect(() => {
    const url = URL.createObjectURL(file);
    const i = new Image();
    i.onload = () => {
      setImg(i);
      setFrame(centred(i, 1));
    };
    i.onerror = () => setFailed(true);
    i.src = url;
    return () => URL.revokeObjectURL(url);
  }, [file]);

  // The scale that makes the picture's short side fill the square.
  const base = img ? view / Math.min(img.naturalWidth, img.naturalHeight) : 1;
  const scale = base * frame.zoom;
  const w = img ? img.naturalWidth * scale : view;
  const h = img ? img.naturalHeight * scale : view;

  // Keep the square covered: the picture can't be moved past its edges.
  const clamp = (f: Frame): Frame => {
    if (!img) return f;
    const s = base * f.zoom;
    const fw = img.naturalWidth * s, fh = img.naturalHeight * s;
    return { zoom: f.zoom, x: Math.min(0, Math.max(view - fw, f.x)), y: Math.min(0, Math.max(view - fh, f.y)) };
  };

  // Zoom around the middle of the square.
  const zoomTo = (z: number) =>
    setFrame((f) => {
      const zoom = Math.min(maxZoom, Math.max(1, z));
      const k = zoom / f.zoom;
      return clamp({ zoom, x: view / 2 - (view / 2 - f.x) * k, y: view / 2 - (view / 2 - f.y) * k });
    });

  function centred(i: HTMLImageElement, zoom: number): Frame {
    const s = (view / Math.min(i.naturalWidth, i.naturalHeight)) * zoom;
    return { zoom, x: (view - i.naturalWidth * s) / 2, y: (view - i.naturalHeight * s) / 2 };
  }

  function onPointerDown(e: PointerEvent<HTMLDivElement>) {
    e.currentTarget.setPointerCapture(e.pointerId);
    drag.current = { px: e.clientX, py: e.clientY, x: frame.x, y: frame.y };
  }
  function onPointerMove(e: PointerEvent<HTMLDivElement>) {
    const d = drag.current;
    if (d) setFrame(clamp({ zoom: frame.zoom, x: d.x + e.clientX - d.px, y: d.y + e.clientY - d.py }));
  }
  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const step = e.shiftKey ? 40 : 10;
    const moves: Record<string, [number, number]> = {
      ArrowLeft: [step, 0],
      ArrowRight: [-step, 0],
      ArrowUp: [0, step],
      ArrowDown: [0, -step],
    };
    if (moves[e.key]) {
      e.preventDefault();
      const [dx, dy] = moves[e.key]!;
      setFrame((f) => clamp({ ...f, x: f.x + dx, y: f.y + dy }));
    } else if (e.key === "+" || e.key === "=") {
      zoomTo(frame.zoom + 0.1);
    } else if (e.key === "-") {
      zoomTo(frame.zoom - 0.1);
    }
  }

  function save() {
    if (!img) return;
    // The part of the picture inside the square, in the picture's pixels.
    const sx = -frame.x / scale, sy = -frame.y / scale, side = view / scale;
    const size = Math.min(outSize, Math.round(side));
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = size;
    const ctx = canvas.getContext("2d")!;
    ctx.imageSmoothingQuality = "high";
    ctx.drawImage(img, sx, sy, side, side, 0, 0, size, size);
    canvas.toBlob((b) => b && onDone(b), "image/jpeg", 0.92);
  }

  return (
    <Dialog
      open
      onClose={onCancel}
      title={m.cropTitle}
      subtitle={m.cropHelp}
      footer={
        <>
          <Button variant="ghost" onClick={onCancel}>
            {m.cancel}
          </Button>
          <Button variant="primary" disabled={!img} onClick={save}>
            {m.savePicture}
          </Button>
        </>
      }
    >
      {failed ? (
        <Alert tone="error">{m.errors.format}</Alert>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 16 }}>
          <div
            role="group"
            aria-label={m.cropArea}
            tabIndex={0}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={() => (drag.current = undefined)}
            onPointerCancel={() => (drag.current = undefined)}
            onWheel={(e) => zoomTo(frame.zoom - e.deltaY / 500)}
            onKeyDown={onKeyDown}
            style={{
              position: "relative",
              width: view,
              height: view,
              overflow: "hidden",
              borderRadius: "var(--fw-r-md)",
              background: "var(--fw-surface-2)",
              cursor: "grab",
              touchAction: "none",
              userSelect: "none",
            }}
          >
            {img && (
              <img
                src={img.src}
                alt=""
                draggable={false}
                style={{ position: "absolute", left: frame.x, top: frame.y, width: w, height: h, maxWidth: "none" }}
              />
            )}
            {/* Pictures show round: everything outside the circle is dimmed. */}
            <div
              aria-hidden="true"
              style={{
                position: "absolute",
                inset: 0,
                borderRadius: "50%",
                boxShadow: "0 0 0 999px rgba(0, 0, 0, 0.55)",
                outline: "2px solid rgba(255, 255, 255, 0.8)",
                pointerEvents: "none",
              }}
            />
          </div>
          <label style={{ display: "flex", alignItems: "center", gap: 12, width: view }}>
            <span className="fw-t-label">{m.zoom}</span>
            <Range
              min={100}
              max={maxZoom * 100}
              value={Math.round(frame.zoom * 100)}
              onChange={(e) => zoomTo(Number(e.currentTarget.value) / 100)}
              style={{ flex: 1 }}
            />
          </label>
        </div>
      )}
    </Dialog>
  );
}
