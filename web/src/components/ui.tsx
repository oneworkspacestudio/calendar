import React, { createContext, useContext, createElement as h } from "react";
import {
  ArrowLeft,
  Video,
  Menu,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  ChevronUp,
  Search,
  CircleHelp,
  Settings,
  CalendarDays,
  CircleCheck,
  Grid3X3,
  Plus,
  X,
  Users,
  StickyNote,
  ContactRound,
  Clock,
  Globe,
  MapPin,
  Bell,
  AlignLeft,
  Trash2,
  Circle,
  Check,
} from "lucide-react";
export const UIContext = createContext<any>({
  dispatch: () => {},
  setField: () => {},
});
const icons: any = {
  menu: Menu,
  "arrow-left": ArrowLeft,
  video: Video,
  "chevron-left": ChevronLeft,
  "chevron-right": ChevronRight,
  "chevron-down": ChevronDown,
  "chevron-up": ChevronUp,
  search: Search,
  "circle-help": CircleHelp,
  settings: Settings,
  "calendar-days": CalendarDays,
  "circle-check": CircleCheck,
  "grid-3x3": Grid3X3,
  plus: Plus,
  x: X,
  users: Users,
  "sticky-note": StickyNote,
  "contact-round": ContactRound,
  clock: Clock,
  globe: Globe,
  "map-pin": MapPin,
  bell: Bell,
  "align-left": AlignLeft,
  "trash-2": Trash2,
  circle: Circle,
  check: Check,
};
export function Icon({ name, color }: any) {
  return h(icons[name] || Circle, {
    size: 20,
    strokeWidth: 1.8,
    "aria-hidden": true,
    style: color ? { color } : undefined,
  });
}
const cls = (type: string, p: any) =>
  [
    type,
    p.role,
    p.variant && "variant-" + p.variant,
    p.selected && "selected",
    p.today && "is-today",
    p.muted && "muted",
  ]
    .filter(Boolean)
    .join(" ");
function native(tag: string, type: string, p: any, extra: any = {}) {
  return h(tag, { className: cls(type, p), ...extra }, p.children ?? p.text);
}
export function Page(p: any) {
  return native("div", "page", p);
}
export function Row(p: any) {
  return native("div", "row", p);
}
export function Stack(p: any) {
  return native("div", "stack", p);
}
export function Card(p: any) {
  return native("section", "card", p, {
    "aria-label": p.role?.replaceAll("-", " "),
    role: p.role?.endsWith("dialog") ? "dialog" : undefined,
    "aria-modal": p.role?.endsWith("dialog") ? true : undefined,
  });
}
export function Grid(p: any) {
  return native("div", "grid", p, { style: { "--cols": p.cols || 7 } as any });
}
export function Section(p: any) {
  const ctx = useContext(UIContext);
  let style: any = {};
  const datum = p.datum;
  const extra: any = {};
  if (p.role === "event") {
    style = {
      top: datum.top,
      height: datum.height,
      left: datum.left,
      width: datum.width,
      background: datum.color,
    };
  }
  if (p.role === "now-line") {
    style = { top: datum.nowTop };
  }
  if (
    p.role === "day-column" ||
    p.role === "month-cell" ||
    p.role === "all-day-column"
  ) {
    extra.onClick = (e: any) => {
      if (e.target.closest("button")) return;
      const rect = e.currentTarget.getBoundingClientRect();
      ctx.dispatch("grid-create", {
        day: datum.date,
        minutes:
          p.role === "day-column"
            ? Math.floor((e.clientY - rect.top) / 24) * 30
            : 9 * 60,
        allDay: p.role !== "day-column",
      });
    };
    extra.onDragOver = (e: any) => e.preventDefault();
    extra.onDrop = (e: any) => {
      e.preventDefault();
      const rect = e.currentTarget.getBoundingClientRect();
      ctx.dispatch("drop-event", {
        href: e.dataTransfer.getData("text/plain"),
        day: datum.date,
        minutes:
          p.role === "day-column"
            ? Math.floor((e.clientY - rect.top) / 24) * 30
            : 9 * 60,
      });
    };
  }
  return native("section", "section", p, { style, ...extra });
}
export function Text(p: any) {
  return native("span", "text", p);
}
export function Heading(p: any) {
  return native("h" + (p.level || 2), "heading", p);
}
export function Spacer() {
  return h("span", { className: "spacer" });
}
export function Divider() {
  return h("hr");
}
export function Image(p: any) {
  return h("img", {
    src: p.src,
    alt: p.alt || "",
    width: p.width,
    height: p.width,
    draggable: false,
  });
}
export function Link(p: any) {
  return native("a", "link", p, { href: p.href, rel: "noreferrer" });
}
export function Button(p: any) {
  const ctx = useContext(UIContext);
  const dispatch = (e: any) => {
    e.stopPropagation();
    if (p.role === "resize-handle") return;
    const actions: any = {
      "mini-date": "date",
      "day-heading": "day",
      "month-number": "day",
      "event-body": "open-event",
      "month-event": "open-event",
      "all-day-event": "open-event",
      "agenda-event": "open-event",
      "contact-row": "open-contact",
    };
    if (actions[p.role]) ctx.dispatch(actions[p.role], p.datum);
    else p.onClick?.(e);
  };
  const extra: any = {
    type: "button",
    className: cls("button", p),
    title: p.title,
    "aria-label": p.title,
    disabled: p.disabled,
    onClick: dispatch,
    "aria-pressed": p.selected || undefined,
    style: p.color ? { "--event-color": p.color } : undefined,
  };
  if (["event-body", "month-event", "all-day-event"].includes(p.role)) {
    extra.draggable = true;
    extra.onDragStart = (e: any) => {
      e.dataTransfer.setData("text/plain", p.datum.key);
      e.dataTransfer.effectAllowed = "move";
    };
  }
  if (p.role === "resize-handle") {
    extra.onPointerDown = (e: any) => {
      e.preventDefault();
      e.stopPropagation();
      const start = e.clientY;
      const finish = (end: any) => {
        window.removeEventListener("pointerup", finish);
        ctx.dispatch("resize-event", {
          event: p.datum,
          minutes: Math.round((end.clientY - start) / 24) * 30,
        });
      };
      window.addEventListener("pointerup", finish, { once: true });
    };
  }
  const children = [];
  if (p.icon) children.push(h(Icon, { key: "icon", name: p.icon }));
  if (p.text !== undefined) children.push(p.text);
  if (p.children) children.push(p.children);
  return h("button", extra, ...children);
}
export function Input(p: any) {
  const ctx = useContext(UIContext);
  const props: any = {
    name: p.field,
    value: p.value ?? "",
    type: p.type || "text",
    placeholder: p.placeholder,
    disabled: p.disabled,
    "aria-label": p.label || p.placeholder,
    onChange: (e: any) => ctx.setField(p.field, e.target.value),
    autoComplete: p.type === "password" ? "new-password" : "off",
  };
  if (p.field === "enginePath") props.spellCheck = false;
  return h(
    "label",
    { className: cls("field", p) },
    p.icon ? h(Icon, { name: p.icon }) : null,
    p.label ? h("span", { className: "field-label" }, p.label) : null,
    h(p.multiline ? "textarea" : "input", props),
  );
}
export function Select(p: any) {
  const ctx = useContext(UIContext);
  return h(
    "label",
    { className: cls("select-field", p) },
    p.label ? h("span", { className: "field-label" }, p.label) : null,
    h(
      "select",
      {
        "aria-label": p.label,
        name: p.field,
        value: p.value ?? "",
        disabled: p.disabled,
        onChange: (e: any) => ctx.setField(p.field, e.target.value),
      },
      ...(p.options || []).map((o: any) =>
        h("option", { key: o.value ?? o, value: o.value ?? o }, o.label ?? o),
      ),
    ),
  );
}
export function Checkbox(p: any) {
  const ctx = useContext(UIContext);
  return h(
    "label",
    {
      className: cls("checkbox", p),
      style: { "--check-color": p.color || "#0b57d0" } as any,
    },
    h("input", {
      type: "checkbox",
      checked: !!p.checked,
      "aria-label": p.label,
      onChange: (e: any) =>
        p.datum
          ? ctx.dispatch("toggle-calendar", p.datum)
          : ctx.setField(p.field, e.target.checked),
    }),
    h("span", {}, p.label),
  );
}
export function Alert(p: any) {
  if (!p.text && !p.children) return null;
  return h(
    "div",
    {
      className: cls("alert", p),
      role: p.tone === "error" ? "alert" : "status",
    },
    p.text,
    p.children,
  );
}
