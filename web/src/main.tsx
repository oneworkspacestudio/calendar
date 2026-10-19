import React, { useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import ICAL from "ical.js";
import {
  option,
  fmt,
  key,
  localDate,
  addDays,
  startWeek,
  hourMinute,
  zoned,
  datesForMonth,
  parseItems,
  emptyForm,
  encode,
} from "./calendar";
import { CalendarScreen } from "./Calendar.generated";
import { UIContext } from "./components/ui";
import "./styles.css";
const base = new URL("./", location.href).pathname.replace(/\/$/, "");
function workspaceTheme() {
  try { const value = localStorage.getItem("workspace-theme"); return value === "dark" ? true : value === "light" ? false : undefined; }
  catch { return undefined; }
}
function App() {
  const [state, set] = useState<any>({
    date: /^\d{4}-\d{2}-\d{2}$/.test(
      new URLSearchParams(location.search).get("date") || "",
    )
      ? localDate(new URLSearchParams(location.search).get("date")!)
      : new Date(),
    mini: new Date(),
    view: ["day", "week", "month", "year", "schedule", "4days"].includes(
      new URLSearchParams(location.search).get("view") || "",
    )
      ? new URLSearchParams(location.search).get("view")
      : "week",
    kind: "",

    sidebar: innerWidth > 900,
    showCalendars: true,
    mode: "calendar",
    calendars: [],
    raw: [],
    contacts: [],
    busy: false,
    signedIn: false,
    error: "",
    toast: "",
    editing: false,
    fullEditor: false,
    createMenu: false,
    existing: null,
    settingsOpen: false,
    contactEditing: false,
    panel: "",
    query: "",
    searching: false,
    setup: location.pathname.endsWith("/setup"),
    form: emptyForm(),
    preferences: {
      weekStart: "1",
      weekends: true,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      dark: workspaceTheme() ?? false,
      eventDuration: "60",
      defaultReminder: "10",
    },
  });
  const stateRef = useRef(state);
  stateRef.current = state;
  useEffect(() => {
    const contacts = state.kind === "contacts" || state.mode === "contacts";
    document.title = (contacts ? "Contacts" : "Calendar") + " · One Workspace";
    const favicon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (favicon) favicon.href = base + (contacts ? "/contacts.svg" : "/calendar.svg");
  }, [state.kind, state.mode]);
  const csrf = useRef("");
  const announced = useRef(new Set<string>());
  const update = (v: any) =>
    set((s: any) => ({ ...s, ...(typeof v === "function" ? v(s) : v) }));
  async function api(path: string, init: any = {}) {
    const response = await fetch(base + path, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": csrf.current,
        ...init.headers,
      },
    });
    if (!response.ok) {
      if (response.status === 401) update({ signedIn: false });
      let text = await response.text();
      try {
        text = JSON.parse(text).error || text;
      } catch {}
      throw new Error(
        (/<html|<!doctype/i.test(text)
          ? `Calendar is temporarily unavailable (HTTP ${response.status}). Retry in a moment.`
          : text.slice(0, 350)) ||
          "Calendar is unavailable. Retry in a moment.",
      );
    }
    return response.json();
  }
  const notice = (message: string) => {
    update({ toast: message });
    setTimeout(() => update({ toast: "" }), 4000);
  };
  async function refresh() {
    update({ busy: true, error: "" });
    try {
      const info = await fetch(base + "/healthz").then((r) => r.json());
      update({ kind: info.kind });
      if (base) {
        const catalog = await fetch('/_workspace/apps').then(r => r.ok ? r.json() : null).catch(() => null);
        if (catalog?.apps) update({workspaceApps: [{title: 'One Workspace', href: '/'}, ...catalog.apps.map((a: any) => ({title: a.Name, href: a.URL, icon: a.Icon}))]});
      }
      const session = await api("/api/session");
      csrf.current = session.csrf;
      const preferences = {
        ...stateRef.current.preferences,
        ...(await api("/api/preferences")),
      };
      preferences.dark = workspaceTheme() ?? preferences.dark;
      const calendars = (await api("/api/calendars")).map((c: any) => ({
        ...c,
        visible:
          stateRef.current.calendars.find((v: any) => v.href === c.href)
            ?.visible ?? !preferences.hiddenCalendars?.includes(c.href),
      }));
      const results = await Promise.all(
        calendars.map((c: any) =>
          api("/api/events?calendar=" + encodeURIComponent(c.href)),
        ),
      );
      const raw = Array.from(
        new Map(results.flat().map((item: any) => [item.href, item])).values(),
      );
      let contacts: any[] = [];
      if (session.kind === "contacts" || stateRef.current.mode === "contacts") {
        try {
          contacts = await api("/api/contacts");
        } catch (error) {
          if (session.kind === "contacts") throw error;
        }
      }
      update({
        signedIn: true,
        demo: session.demo,
        session,
        calendars,
        raw,
        contacts,
        preferences,
        mode: session.kind === "contacts" ? "contacts" : stateRef.current.mode,
      });
    } catch (error: any) {
      if (stateRef.current.signedIn) update({ error: error.message });
    } finally {
      update({ busy: false });
    }
  }
  useEffect(() => {
    if (state.setup) {
      fetch(base + "/api/setup-info", {
        headers: { "X-Setup-Token": location.hash.slice(1) },
      })
        .then((r) => r.json())
        .then((info) => update({ form: { ...emptyForm(), ...info } }))
        .catch(() => {});
    } else void refresh();
  }, []);
  const model = useMemo(() => {
    const first = Number(state.preferences.weekStart),
      zone =
        state.preferences.timezone ||
        Intl.DateTimeFormat().resolvedOptions().timeZone;
    let dates: Date[] = [];
    if (state.view === "day") dates = [state.date];
    else if (state.view === "4days")
      dates = Array.from({ length: 4 }, (_, i) => addDays(state.date, i));
    else
      dates = Array.from({ length: 7 }, (_, i) =>
        addDays(startWeek(state.date, first), i),
      ).filter(
        (d) =>
          state.preferences.weekends || (d.getDay() !== 0 && d.getDay() !== 6),
      );
    let rangeStart = addDays(state.date, -40),
      rangeEnd = addDays(state.date, 50);
    if (state.view === "year") {
      rangeStart = new Date(state.date.getFullYear(), 0, 1);
      rangeEnd = new Date(state.date.getFullYear() + 1, 0, 1);
    }
    const events = parseItems(state.raw, state.calendars, rangeStart, rangeEnd)
      .filter(
        (e) =>
          state.calendars.find((c: any) => c.href === e.calendar)?.visible !==
          false,
      )
      .filter((e) => state.mode !== "tasks" || e.task)
      .filter(
        (e) =>
          !state.query ||
          [
            e.title,
            e.component.getFirstPropertyValue("description"),
            e.component.getFirstPropertyValue("location"),
            ...e.component
              .getAllProperties("attendee")
              .map((p: any) => p.getFirstValue()),
          ]
            .join(" ")
            .toLowerCase()
            .includes(state.query.toLowerCase()),
      );
    const decorate = (e: any) => ({
      ...e,
      dayLabel: fmt(
        e.start,
        { weekday: "short", day: "numeric", month: "short" },
        zone,
      ),
      timeLabel: e.allDay
        ? "All day"
        : fmt(e.start, { hour: "numeric", minute: "2-digit" }, zone) +
          " – " +
          fmt(e.end, { hour: "numeric", minute: "2-digit" }, zone),
      shortTime: e.allDay
        ? ""
        : fmt(e.start, { hour: "numeric", minute: "2-digit" }, zone),
    });
    const dayData = (d: Date) => {
      const date = key(d),
        dayStart = zoned(date, "00:00", zone),
        dayEnd = zoned(key(addDays(d, 1)), "00:00", zone);
      const matches = events
        .filter((e) => e.start < dayEnd && e.end > dayStart)
        .sort((a, b) => +a.start - +b.start)
        .map(decorate);
      const timed = matches.filter((e) => !e.allDay);
      const columns: any[][] = [];
      for (const event of timed) {
        let col = columns.findIndex(
          (list) => list[list.length - 1].end <= event.start,
        );
        if (col < 0) {
          col = columns.length;
          columns.push([]);
        }
        columns[col].push(event);
        event.column = col;
      }
      const count = Math.max(columns.length, 1);
      return {
        date,
        number: d.getDate(),
        label: fmt(d, { weekday: "short" }),
        today: date === key(new Date(), zone),
        allDay: matches.filter((e) => e.allDay),
        allEvents: matches,
        events: timed.map((e) => ({
          ...e,
          top: Math.max(
            0,
            (e.start < dayStart ? 0 : hourMinute(e.start, zone)) * 0.8,
          ),
          height: Math.max(
            23,
            Math.min(
              24 * 60,
              (+e.end - Math.max(+e.start, +dayStart)) / 60000,
            ) *
              0.8 -
              2,
          ),
          left: `calc(${(e.column / count) * 100}% + 2px)`,
          width: `calc(${100 / count}% - 5px)`,
        })),
        showNow: date === key(new Date(), zone),
        nowTop: hourMinute(new Date(), zone) * 0.8,
      };
    };
    const monthCount =
      Math.ceil(
        (((new Date(
          state.date.getFullYear(),
          state.date.getMonth(),
          1,
        ).getDay() -
          first +
          7) %
          7) +
          new Date(
            state.date.getFullYear(),
            state.date.getMonth() + 1,
            0,
          ).getDate()) /
          7,
      ) * 7;
    const monthDays = datesForMonth(state.date, first)
      .slice(0, monthCount)
      .filter(
        (d) =>
          state.preferences.weekends ||
          ![0, 6].includes(localDate(d.date).getDay()),
      )
      .map((d) => ({ ...dayData(localDate(d.date)), muted: d.muted }));
    const miniDays = datesForMonth(state.mini, first).map((d) => ({
      ...d,
      selected: d.date === key(state.date),
    }));
    const weekdays = Array.from(
      { length: 7 },
      (_, i) => new Date(2026, 0, 4 + ((i + first) % 7)),
    );
    const agenda = events.map(decorate).sort((a, b) => +a.start - +b.start);
    let period = fmt(state.date, { month: "long", year: "numeric" });
    if (innerWidth < 600)
      period = fmt(state.date, { month: "short", year: "numeric" });
    if (state.view === "year") period = String(state.date.getFullYear());
    if (state.view === "day")
      period = fmt(state.date, {
        day: "numeric",
        month: "long",
        year: "numeric",
      });
    let panelTitle = "",
      panelText = "";
    if (state.panel === "service") {
      panelTitle = "Shared service";
      panelText = state.serviceInfo
        ? JSON.stringify(state.serviceInfo, null, 2)
        : "Checking the shared backend…";
    }
    if (state.panel === "help") {
      panelTitle = "Calendar help";
      panelText =
        "Create an event with Create or click a time on the calendar. Drag an event to move it, or drag its bottom edge to change its duration.\n\nKeyboard shortcuts: C creates an event; T returns to today; J/K move between periods; D/W/M/Y/A change views; / opens search; Esc closes dialogs.\n\nRepeating events can be edited for this occurrence or the entire series. Settings includes iCalendar import/export and local reminders.";
    }
    if (state.panel === "account") {
      panelTitle = state.session?.name || "Your account";
      panelText = state.session?.email || "Signed in with MyAccounts";
    }
    if (state.panel === "apps") {
      panelTitle = "One Workspace";
      panelText = "Open another application in your workspace.";
    }
    if (state.panel === "booking") {
      panelTitle = "Appointment schedules";
      panelText =
        "Create availability blocks as appointment events on your calendar. Public booking links and invitations are not enabled by default.";
    }
    if (state.panel === "notes") {
      panelTitle = "Keep a note";
      panelText = "Private notes are saved with your account preferences.";
    }
    const contacts = state.contacts.map((raw: any) => {
      try {
        const c = new ICAL.Component(ICAL.parse(raw.data));
        return {
          ...raw,
          name: c.getFirstPropertyValue("fn") || "Unnamed contact",
          email: c.getFirstPropertyValue("email") || "",
          phone: c.getFirstPropertyValue("tel") || "",
          company: c.getFirstPropertyValue("org") || "",
          initial: String(c.getFirstPropertyValue("fn") || "?").slice(0, 1),
        };
      } catch {
        return {
          ...raw,
          name: "Unreadable contact",
          email: "",
          phone: "",
          initial: "?",
        };
      }
    });
    const visibleContacts = contacts.filter((contact: any) => !state.query || [contact.name, contact.email, contact.phone, contact.company].join(" ").toLocaleLowerCase().includes(state.query.toLocaleLowerCase()));
    const showCalendar = state.mode !== "contacts";
    return {
      ...state,
      showCalendar,
      configLoaded: Boolean(state.kind),
      calendarSearching: showCalendar && state.searching,
      workspaceHome: base ? "/" : "https://github.com/oneworkspacestudio/one-workspace",
      searchPlaceholder: showCalendar ? "Search Calendar" : "Search Contacts",
      syncLabel: showCalendar ? "Syncing your calendar…" : "Syncing your contacts…",
      storageHint: state.kind === "contacts" ? "Your contacts stay on your own shared Stalwart service." : "Your calendars stay on your own shared Stalwart service.",
      editorOverlay: state.fullEditor
        ? "modal-overlay full-editor"
        : "modal-overlay event-overlay",
      contacts: visibleContacts,
      appTitle:
        state.mode === "contacts" || state.kind === "contacts"
          ? "Contacts"
          : "Calendar",
      appIcon:
        state.mode === "contacts" || state.kind === "contacts"
          ? "contacts.svg"
          : "calendar.svg",
      loginTitle:
        state.kind === "contacts"
          ? "Your people, together"
          : "Your day, together",
      loginDescription:
        "Sign in to My " +
        (state.kind === "contacts" ? "Contacts" : "Calendar") +
        " with your MyAccounts account.",
      monthColumns: state.preferences.weekends ? 7 : 5,
      calendarMode: state.mode === "calendar",
      tasksMode: state.mode === "tasks",
      contactsMode: state.mode === "contacts",
      days: dates.map(dayData),
      columnCount: dates.length + 1,
      monthDays,
      miniDays,
      miniTitle: fmt(state.mini, { month: "long", year: "numeric" }),
      period,
      weekdayLetters: weekdays.map((d) => fmt(d, { weekday: "narrow" })),
      weekdayNames: weekdays
        .filter(
          (d) => state.preferences.weekends || ![0, 6].includes(d.getDay()),
        )
        .map((d) => fmt(d, { weekday: "short" })),
      yearMonths: Array.from({ length: 12 }, (_, i) => ({
        title: fmt(new Date(state.date.getFullYear(), i, 1), { month: "long" }),
        days: datesForMonth(new Date(state.date.getFullYear(), i, 1), first),
      })),
      hours: Array.from(
        { length: 24 },
        (_, i) => `${String(i).padStart(2, "0")}:00`,
      ),
      timezoneLabel:
        "GMT" +
        fmt(new Date(), { timeZoneName: "shortOffset" }, zone)
          .split(" ")
          .pop()
          ?.replace("GMT", ""),
      weekView:
        showCalendar &&
        ["week", "day", "4days"].includes(state.view) &&
        state.mode !== "tasks",
      monthView:
        showCalendar && state.view === "month" && state.mode !== "tasks",
      yearView: showCalendar && state.view === "year" && state.mode !== "tasks",
      scheduleView:
        showCalendar && (state.view === "schedule" || state.mode === "tasks"),
      agenda,
      empty: agenda.length === 0,
      initial: state.session?.name?.slice(0, 1) || "A",
      viewOptions: [
        option("day", "Day"),
        option("week", "Week"),
        option("month", "Month"),
        option("year", "Year"),
        option("schedule", "Schedule"),
        option("4days", "4 days"),
      ],
      durationOptions: [
        option("15", "15 minutes"),
        option("30", "30 minutes"),
        option("60", "60 minutes"),
        option("90", "90 minutes"),
        option("120", "120 minutes"),
      ],
      repeatOptions: [
        option("none", "Does not repeat"),
        option("DAILY", "Every day"),
        option("WEEKLY", "Every week"),
        option("MONTHLY", "Every month"),
        option("YEARLY", "Every year"),
        option("custom", "Keep custom recurrence"),
      ],
      scopeOptions: [
        option("event", "This event"),
        option("series", "Entire series"),
      ],
      reminderOptions: [
        option("none", "No notification"),
        option("0", "At time of event"),
        option("5", "5 minutes before"),
        option("10", "10 minutes before"),
        option("30", "30 minutes before"),
        option("60", "1 hour before"),
        option("1440", "1 day before"),
      ],
      availabilityOptions: [option("busy", "Busy"), option("free", "Free")],
      visibilityOptions: [
        option("default", "Default visibility"),
        option("public", "Public"),
        option("private", "Private"),
      ],
      weekStartOptions: [
        option("0", "Sunday"),
        option("1", "Monday"),
        option("6", "Saturday"),
      ],
      calendarOptions: state.calendars.map((c: any) => option(c.href, c.name)),
      timezones: Array.from(
        new Set([
          zone,
          "Europe/London",
          "America/New_York",
          "America/Los_Angeles",
          "Europe/Paris",
          "Asia/Dubai",
          "Asia/Kolkata",
          "Asia/Tokyo",
          "Australia/Sydney",
          "UTC",
        ]),
      ).map((z) => option(z)),
      repeating: !!state.existing?.recurring,
      eventType: state.form.type === "event",
      taskType: state.form.type === "task",
      appointmentType: state.form.type === "appointment",
      panelTitle,
      panelText,
      notesPanel: state.panel === "notes",
      accountPanel: state.panel === "account",
      appLinks: state.panel === "apps" ? (state.workspaceApps || [
        {title: "Source code", href: "https://github.com/oneworkspacestudio/calendar"}
      ]) : [],
      callback: location.origin + base + "/auth/callback",
    };
  }, [state]);
  const modelRef = useRef(model);
  modelRef.current = model;
  useEffect(() => {
    document.documentElement.classList.toggle("dark", !!state.preferences.dark);
    document.documentElement.dataset.theme = state.preferences.dark ? "dark" : "light";
    const timer = setTimeout(() => {
      const el = document.querySelector(".time-scroll");
      if (el && el.scrollTop === 0) el.scrollTop = 7 * 48;
    }, 30);
    return () => clearTimeout(timer);
  }, [state.view, state.signedIn, state.preferences.dark]);
  useEffect(() => {
    const sync = (e: StorageEvent) => { if(e.key === "workspace-theme") update((s: any) => ({preferences: {...s.preferences, dark: e.newValue === "dark"}})); };
    window.addEventListener("storage", sync);
    return () => window.removeEventListener("storage", sync);
  }, []);
  useEffect(() => {
    const timer = setInterval(() => update({ clock: Date.now() }), 60000);
    return () => clearInterval(timer);
  }, []);
  useEffect(() => {
    if (state.editing)
      document
        .querySelector<HTMLInputElement>(".event-title-input input")
        ?.focus();
  }, [state.editing]);
  useEffect(() => {
    if (!state.setup) {
      const url = new URL(location.href);
      url.searchParams.set("view", state.view);
      url.searchParams.set("date", key(state.date));
      history.replaceState(null, "", url);
    }
  }, [state.view, state.date]);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        dispatch("escape");
        return;
      }
      if (
        (e.target as HTMLElement).closest("input,textarea,select") ||
        e.ctrlKey ||
        e.metaKey
      )
        return;
      const actions: any = {
        c: "create",
        t: "today",
        j: "next",
        k: "previous",
        "/": "search",
        Escape: "escape",
      };
      const views: any = {
        d: "day",
        w: "week",
        m: "month",
        y: "year",
        a: "schedule",
      };
      if (views[e.key]) {
        e.preventDefault();
        update({ view: views[e.key] });
      } else if (actions[e.key]) {
        e.preventDefault();
        dispatch(actions[e.key]);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);
  useEffect(() => {
    const timer = setInterval(() => {
      if (
        typeof Notification === "undefined" ||
        Notification.permission !== "granted"
      )
        return;
      const current = modelRef.current;
      for (const event of current.agenda || []) {
        const alarm = event.component.getFirstSubcomponent("valarm");
        if (!alarm) continue;
        const trigger = alarm.getFirstPropertyValue("trigger");
        const when = +event.start + (trigger?.toSeconds?.() || 0) * 1000;
        const now = Date.now();
        if (
          now >= when &&
          now < when + 60000 &&
          !announced.current.has(event.key)
        ) {
          announced.current.add(event.key);
          new Notification(event.title, {
            body: event.timeLabel,
            tag: event.key,
          });
        }
      }
    }, 15000);
    return () => clearInterval(timer);
  }, []);
  function setField(field: string, value: any) {
    if (field === "view") update({ view: value });
    else if (field === "query") update({ query: value, searching: !!value });
    else
      update((s: any) => {
        const form = { ...s.form, [field]: value };
        if (
          field === "startDate" &&
          /^\d{4}-\d{2}-\d{2}$/.test(value) &&
          s.form.endDate
        ) {
          const days = Math.round(
            (+localDate(value) - +localDate(s.form.startDate)) / 86400000,
          );
          form.endDate = key(addDays(localDate(s.form.endDate), days));
        }
        if (
          field === "startTime" &&
          value &&
          s.form.startDate &&
          s.form.endDate
        ) {
          const duration =
            +zoned(s.form.endDate, s.form.endTime, s.form.timezone) -
            +zoned(s.form.startDate, s.form.startTime, s.form.timezone);
          const end = new Date(
            +zoned(s.form.startDate, value, s.form.timezone) +
              Math.max(1800000, duration),
          );
          form.endDate = key(end, s.form.timezone);
          form.endTime = fmt(
            end,
            { hour: "2-digit", minute: "2-digit" },
            s.form.timezone,
          );
        }
        return { form };
      });
  }
  function openEvent(event: any) {
    const c = event.component;
    const start = event.start,
      end = event.end;
    const recurrence = c.getFirstPropertyValue("rrule");
    const guests = c
      .getAllProperties("attendee")
      .map((p: any) => String(p.getFirstValue()).replace(/^mailto:/i, ""))
      .join(", ");
    const form = {
      ...emptyForm(),
      title: event.title === "(No title)" ? "" : event.title,
      startDate: key(start, stateRef.current.preferences.timezone),
      endDate: key(
        event.allDay ? addDays(end, -1) : end,
        stateRef.current.preferences.timezone,
      ),
      startTime: fmt(
        start,
        { hour: "2-digit", minute: "2-digit" },
        stateRef.current.preferences.timezone,
      ),
      endTime: fmt(
        end,
        { hour: "2-digit", minute: "2-digit" },
        stateRef.current.preferences.timezone,
      ),
      allDay: event.allDay,
      calendar: event.calendar,
      meetingUrl: c.getFirstPropertyValue("url") || "",
      location: c.getFirstPropertyValue("location") || "",
      description: c.getFirstPropertyValue("description") || "",
      guests,
      recurrence: recurrence
        ? recurrence.toString() === "FREQ=" + recurrence.freq
          ? recurrence.freq
          : "custom"
        : "none",
      scope: event.recurring ? "event" : "series",
      timezone: stateRef.current.preferences.timezone,
      reminder: c.getFirstSubcomponent("valarm")
        ? String(
            -(
              c
                .getFirstSubcomponent("valarm")
                .getFirstPropertyValue("trigger")
                ?.toSeconds?.() || 0
            ) / 60,
          )
        : "none",
      availability:
        c.getFirstPropertyValue("transp") === "TRANSPARENT" ? "free" : "busy",
      visibility: String(
        c.getFirstPropertyValue("class") || "default",
      ).toLowerCase(),
      type: event.task
        ? "task"
        : c.getFirstPropertyValue("x-workspace-appointment") === "true"
          ? "appointment"
          : "event",
      completed: event.complete,
    };
    update({ editing: true, existing: event, form, error: "" });
    return form;
  }
  async function saveEvent(
    form = stateRef.current.form,
    existing = stateRef.current.existing,
  ) {
    update({ busy: true, error: "" });
    try {
      const data = encode(form, existing);
      const href = existing?.href || form.calendar + form.uid + ".ics";
      if (!form.calendar) throw new Error("Choose a calendar before saving.");
      await api("/api/events", {
        method: "PUT",
        body: JSON.stringify({ href, etag: existing?.etag || "*", data }),
      });
      update({ editing: false, existing: null });
      await refresh();
      notice("Event saved");
    } catch (error: any) {
      update({ error: error.message });
    } finally {
      update({ busy: false });
    }
  }
  async function saveContact() {
    const s = stateRef.current,
      f = s.form;
    const root = s.existing
      ? new ICAL.Component(ICAL.parse(s.existing.data))
      : new ICAL.Component("vcard");
    root.updatePropertyWithValue("version", "3.0");
    root.updatePropertyWithValue(
      "uid",
      root.getFirstPropertyValue("uid") || f.uid || crypto.randomUUID(),
    );
    for (const [prop, value] of Object.entries({
      fn: f.name || "Unnamed contact",
      email: f.email,
      tel: f.phone,
      org: f.company,
    }))
      root.updatePropertyWithValue(prop, value || "");
    const subject = s.session.subject + "@" + s.session.domain;
    const href =
      s.existing?.href ||
      "/dav/card/" +
        encodeURIComponent(subject) +
        "/default/" +
        (f.uid || root.getFirstPropertyValue("uid")) +
        ".vcf";
    await api("/api/contacts", {
      method: "PUT",
      body: JSON.stringify({
        href,
        etag: s.existing?.etag || "*",
        data: root.toString() + "\r\n",
      }),
    });
    update({ contactEditing: false });
    await refresh();
    notice("Contact saved");
  }
  async function dispatch(action: string, payload?: any) {
    const s = stateRef.current,
      m = modelRef.current;
    try {
      switch (action) {
        case "escape":
          update({
            createMenu: false,
            editing: false,
            settingsOpen: false,
            panel: "",
            contactEditing: false,
          });
          break;
        case "toggle-sidebar":
          update({ sidebar: !s.sidebar });
          break;
        case "toggle-calendars":
          update({ showCalendars: !s.showCalendars });
          break;
        case "today":
          update({ date: new Date(), mini: new Date() });
          break;
        case "previous":
        case "next": {
          const sign = action === "next" ? 1 : -1;
          const date =
            s.view === "month"
              ? new Date(s.date.getFullYear(), s.date.getMonth() + sign, 1, 12)
              : s.view === "year"
                ? new Date(s.date.getFullYear() + sign, 0, 1, 12)
                : addDays(
                    s.date,
                    sign * (s.view === "day" ? 1 : s.view === "4days" ? 4 : 7),
                  );
          update({ date, mini: date });
          break;
        }
        case "mini-previous":
        case "mini-next":
          update({
            mini: new Date(
              s.mini.getFullYear(),
              s.mini.getMonth() + (action === "mini-next" ? 1 : -1),
              1,
              12,
            ),
          });
          break;
        case "date":
        case "day":
          update({
            date: localDate(payload.date),
            mini: localDate(payload.date),
            ...(action === "day" ? { view: "day" } : {}),
            ...(innerWidth < 900 ? { sidebar: false } : {}),
          });
          break;
        case "toggle-create":
          update({ createMenu: !s.createMenu });
          break;
        case "more-options":
          update({ fullEditor: true });
          break;
        case "create-task":
        case "create-appointment":
        case "create":
        case "grid-create": {
          const form = emptyForm(
            payload?.day || key(s.date),
            payload?.minutes ??
              (key(s.date) === key(new Date())
                ? Math.min(
                    1380,
                    Math.ceil(
                      (new Date().getHours() * 60 + new Date().getMinutes()) /
                        30,
                    ) * 30,
                  )
                : 9 * 60),
            payload?.allDay || false,
            s.calendars[0]?.href || "",
          );
          form.timezone = s.preferences.timezone;
          form.reminder = s.preferences.defaultReminder || "10";
          if (!form.allDay) {
            const end = new Date(
              +zoned(form.startDate, form.startTime, form.timezone) +
                Number(s.preferences.eventDuration || 60) * 60000,
            );
            form.endDate = key(end, form.timezone);
            form.endTime = fmt(
              end,
              { hour: "2-digit", minute: "2-digit" },
              form.timezone,
            );
          }
          if (action === "create-task") form.type = "task";
          if (action === "create-appointment") form.type = "appointment";
          update({
            editing: true,
            createMenu: false,
            fullEditor: false,
            existing: null,
            form,
            error: "",
          });
          break;
        }
        case "open-event":
          openEvent(payload);
          break;
        case "close-editor":
          update({ editing: false, error: "" });
          break;
        case "save-event":
          await saveEvent();
          break;
        case "delete-event": {
          if (!s.existing) break;
          if (
            !confirm(
              s.existing.recurring
                ? "Delete the entire repeating series?"
                : "Delete this event?",
            )
          )
            break;
          await api("/api/events", {
            method: "DELETE",
            body: JSON.stringify({
              href: s.existing.href,
              etag: s.existing.etag,
            }),
          });
          update({ editing: false });
          await refresh();
          notice("Event deleted");
          break;
        }
        case "drop-event":
        case "resize-event": {
          const event =
            action === "drop-event"
              ? m.agenda.find((e: any) => e.key === payload.href)
              : payload.event;
          if (!event) break;
          const eventForm = openEvent(event);
          const newStart =
            action === "drop-event"
              ? zoned(
                  payload.day,
                  String(Math.floor(payload.minutes / 60)).padStart(2, "0") +
                    ":" +
                    String(payload.minutes % 60).padStart(2, "0"),
                  s.preferences.timezone,
                )
              : event.start;
          const newEnd =
            action === "drop-event"
              ? new Date(+newStart + (+event.end - +event.start))
              : new Date(+event.end + payload.minutes * 60000);
          const form = {
            ...eventForm,
            title: event.title,
            calendar: event.calendar,
            startDate: key(newStart, s.preferences.timezone),
            endDate: key(
              event.allDay ? addDays(newEnd, -1) : newEnd,
              s.preferences.timezone,
            ),
            startTime: fmt(
              newStart,
              { hour: "2-digit", minute: "2-digit" },
              s.preferences.timezone,
            ),
            endTime: fmt(
              newEnd,
              { hour: "2-digit", minute: "2-digit" },
              s.preferences.timezone,
            ),
            timezone: s.preferences.timezone,
            allDay: event.allDay,
            location: event.component.getFirstPropertyValue("location") || "",
            description:
              event.component.getFirstPropertyValue("description") || "",
            guests: event.component
              .getAllProperties("attendee")
              .map((p: any) =>
                String(p.getFirstValue()).replace(/^mailto:/, ""),
              )
              .join(", "),
            recurrence: "custom",
            scope: event.recurring ? "event" : "series",
          };
          await saveEvent(form, event);
          break;
        }
        case "toggle-calendar": {
          const calendars = s.calendars.map((c: any) =>
            c.href === payload.href ? { ...c, visible: !c.visible } : c,
          );
          const preferences = {
            ...s.preferences,
            hiddenCalendars: calendars
              .filter((c: any) => !c.visible)
              .map((c: any) => c.href),
          };
          update({ calendars, preferences });
          await api("/api/preferences", {
            method: "PUT",
            body: JSON.stringify(preferences),
          });
          break;
        }
        case "new-calendar": {
          const name = prompt("Name your new calendar");
          if (name) {
            await api("/api/calendars", {
              method: "POST",
              body: JSON.stringify({ name, color: "#1a73e8" }),
            });
            await refresh();
          }
          break;
        }
        case "search":
          update({ searching: true, query: "" });
          setTimeout(
            () =>
              document
                .querySelector<HTMLInputElement>(".search input")
                ?.focus(),
            0,
          );
          break;
        case "close-search":
          update({ searching: false, query: "" });
          break;
        case "settings-timezone":
        case "settings-events":
        case "settings-view":
        case "settings-import": {
          const target: any = {
            "settings-timezone": ".timezone-settings",
            "settings-events": ".event-settings",
            "settings-view": ".view-settings",
            "settings-import": ".import-settings",
          };
          document
            .querySelector(target[action])
            ?.scrollIntoView({ behavior: "smooth", block: "start" });
          break;
        }
        case "settings":
          update({ settingsOpen: true, form: { ...s.preferences } });
          break;
        case "close-settings":
          update({ settingsOpen: false });
          break;
        case "save-settings": {
          try {
            new Intl.DateTimeFormat("en-GB", { timeZone: s.form.timezone });
          } catch {
            throw new Error("Choose a valid time zone such as Europe/London.");
          }
          const preferences = {
            ...s.preferences,
            ...s.form,
            hiddenCalendars: s.calendars
              .filter((c: any) => !c.visible)
              .map((c: any) => c.href),
          };
          await api("/api/preferences", {
            method: "PUT",
            body: JSON.stringify(preferences),
          });
          update({ preferences, settingsOpen: false });
          try { localStorage.setItem("workspace-theme", preferences.dark ? "dark" : "light"); } catch {}
          notice("Settings saved");
          break;
        }
        case "calendar-mode":
          update({ mode: "calendar" });
          break;
        case "tasks-mode":
          update({ mode: "tasks" });
          break;
        case "contacts":
          update({ mode: "contacts", contacts: await api("/api/contacts") });
          break;
        case "create-contact":
          update({
            contactEditing: true,
            existing: null,
            form: {
              name: "",
              email: "",
              phone: "",
              company: "",
              uid: crypto.randomUUID(),
            },
          });
          break;
        case "open-contact":
          update({ contactEditing: true, existing: payload, form: payload });
          break;
        case "close-contact":
          update({ contactEditing: false });
          break;
        case "save-contact":
          await saveContact();
          break;
        case "delete-contact":
          if (confirm("Delete this contact?")) {
            await api("/api/contacts", {
              method: "DELETE",
              body: JSON.stringify({
                href: s.existing.href,
                etag: s.existing.etag,
              }),
            });
            update({ contactEditing: false });
            await refresh();
            notice("Contact deleted");
          }
          break;
        case "help":
        case "account":
        case "booking":
        case "notes":
          update({ panel: action, form: { note: s.preferences.note || "" } });
          break;
        case "apps":
          if (document.documentElement.dataset.workspaceNavigation === "ready") window.dispatchEvent(new Event("workspace:open-apps"));
          else update({ panel: "apps" });
          break;
        case "save-notes": {
          const preferences = { ...s.preferences, note: s.form.note };
          await api("/api/preferences", {
            method: "PUT",
            body: JSON.stringify(preferences),
          });
          update({ preferences, panel: "" });
          notice("Note saved");
          break;
        }
        case "signout":
          await api("/auth/logout", { method: "POST" });
          update({ signedIn: false, panel: "" });
          break;
        case "close-panel":
          update({ panel: "" });
          break;
        case "service-status":
          update({ panel: "service", serviceInfo: await api("/api/backend") });
          break;
        case "toggle-rail":
          document.querySelector(".right-rail")?.classList.toggle("collapsed");
          break;
        case "type-event":
        case "type-task":
        case "type-appointment":
          update({ form: { ...s.form, type: action.slice(5) } });
          break;
        case "refresh":
          await refresh();
          break;
        case "notifications":
          if ("Notification" in window) {
            const p = await Notification.requestPermission();
            notice(
              p === "granted"
                ? "Local calendar reminders enabled"
                : "Notifications were not enabled",
            );
          }
          break;
        case "import": {
          const input = document.createElement("input");
          input.type = "file";
          input.accept = ".ics,text/calendar";
          input.onchange = async () => {
            try {
              const file = input.files?.[0];
              if (!file) return;
              if (file.size > 5 * 1024 * 1024)
                throw new Error("Import a calendar smaller than 5 MB.");
              const root = new ICAL.Component(ICAL.parse(await file.text()));
              const groups = new Map<string, any[]>();
              for (const c of root.getAllSubcomponents("vevent")) {
                const uid =
                  c.getFirstPropertyValue("uid") || crypto.randomUUID();
                groups.set(uid, [...(groups.get(uid) || []), c]);
              }
              if (!s.calendars[0])
                throw new Error("Create a calendar before importing.");
              update({ busy: true });
              for (const components of groups.values()) {
                const calendar = new ICAL.Component("vcalendar");
                calendar.updatePropertyWithValue("version", "2.0");
                for (const zone of root.getAllSubcomponents("vtimezone"))
                  calendar.addSubcomponent(
                    new ICAL.Component(structuredClone(zone.toJSON())),
                  );
                for (const c of components) calendar.addSubcomponent(c);
                await api("/api/events", {
                  method: "PUT",
                  body: JSON.stringify({
                    href: s.calendars[0].href + crypto.randomUUID() + ".ics",
                    etag: "*",
                    data: calendar.toString() + "\r\n",
                  }),
                });
              }
              await refresh();
              notice(`${groups.size} events imported`);
            } catch (error: any) {
              update({ error: error.message, busy: false });
            }
          };
          input.click();
          break;
        }
        case "export": {
          const root = new ICAL.Component("vcalendar");
          root.updatePropertyWithValue("version", "2.0");
          root.updatePropertyWithValue(
            "prodid",
            "-//One Workspace//Calendar//EN",
          );
          for (const raw of s.raw) {
            const calendar = new ICAL.Component(ICAL.parse(raw.data));
            for (const c of calendar.getAllSubcomponents())
              root.addSubcomponent(
                new ICAL.Component(structuredClone(c.toJSON())),
              );
          }
          const url = URL.createObjectURL(
            new Blob([root.toString() + "\r\n"], { type: "text/calendar" }),
          );
          const link = document.createElement("a");
          link.href = url;
          link.download = "my-calendar.ics";
          link.click();
          setTimeout(() => URL.revokeObjectURL(url), 1000);
          break;
        }
        case "setup-save": {
          update({ busy: true, error: "" });
          await api("/api/setup", {
            method: "POST",
            headers: { "X-Setup-Token": location.hash.slice(1) },
            body: JSON.stringify(s.form),
          });
          location.replace(base + "/");
          break;
        }
      }
    } catch (error: any) {
      update({ error: error.message, busy: false });
    }
  }
  return React.createElement(
    UIContext.Provider,
    { value: { dispatch, setField } },
    React.createElement(CalendarScreen, { data: model, onAction: dispatch }),
  );
}
createRoot(document.getElementById("root")!).render(React.createElement(App));
