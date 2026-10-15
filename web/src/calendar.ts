import ICAL from "ical.js";
const option = (value: any, label?: string) => ({
  value,
  label: label || value,
});
const fmt = (date: any, opts: any = {}, zone?: string) =>
  new Intl.DateTimeFormat("en-GB", {
    ...opts,
    ...(zone ? { timeZone: zone } : {}),
  }).format(new Date(date));
const key = (date: any, zone?: string) => {
  const p = new Intl.DateTimeFormat("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    ...(zone ? { timeZone: zone } : {}),
  }).formatToParts(new Date(date));
  return ["year", "month", "day"]
    .map((t) => p.find((v) => v.type === t)?.value)
    .join("-");
};
const localDate = (s: string) => new Date(s + "T12:00:00");
const addDays = (d: Date, n: number) =>
  new Date(d.getFullYear(), d.getMonth(), d.getDate() + n, 12);
const startWeek = (d: Date, first: number) =>
  addDays(d, -(d.getDay() - first + 7) % 7);
const hourMinute = (d: Date, zone: string) => {
  const p = new Intl.DateTimeFormat("en-GB", {
    timeZone: zone,
    hourCycle: "h23",
    hour: "2-digit",
    minute: "2-digit",
  }).formatToParts(d);
  return (
    Number(p.find((p) => p.type === "hour")?.value) * 60 +
    Number(p.find((p) => p.type === "minute")?.value)
  );
};
function zoned(date: string, time: string, zone: string) {
  const wall = new Date(date + "T" + (time || "00:00") + ":00Z");
  let guess = wall.getTime();
  for (let n = 0; n < 3; n++) {
    const p = new Intl.DateTimeFormat("en-CA", {
      timeZone: zone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23",
    }).formatToParts(new Date(guess));
    const v = (t: string) => Number(p.find((a) => a.type === t)?.value);
    const shown = Date.UTC(
      v("year"),
      v("month") - 1,
      v("day"),
      v("hour"),
      v("minute"),
      v("second"),
    );
    guess += wall.getTime() - shown;
  }
  return new Date(guess);
}
function datesForMonth(d: Date, first: number) {
  const firstDay = new Date(d.getFullYear(), d.getMonth(), 1, 12);
  const start = startWeek(firstDay, first);
  return Array.from({ length: 42 }, (_, i) => {
    const date = addDays(start, i);
    return {
      date: key(date),
      day: date.getDate(),
      number: date.getDate(),
      muted: date.getMonth() !== d.getMonth(),
      today: key(date) === key(new Date()),
    };
  });
}
function parseItems(
  items: any[],
  calendars: any[],
  rangeStart: Date,
  rangeEnd: Date,
) {
  const result: any[] = [];
  for (const raw of items) {
    try {
      const calendar =
        calendars.find((c) => raw.href.startsWith(c.href)) || calendars[0];
      const root = new ICAL.Component(ICAL.parse(raw.data));
      for (const component of root.getAllSubcomponents("vtimezone"))
        ICAL.TimezoneService.register(new ICAL.Timezone(component));
      const components = root.getAllSubcomponents("vevent");
      for (const component of components.filter(
        (c: any) => !c.hasProperty("recurrence-id"),
      )) {
        const ev = new ICAL.Event(component);
        for (const ex of components.filter(
          (c: any) =>
            c.hasProperty("recurrence-id") &&
            c.getFirstPropertyValue("uid") === ev.uid,
        )) {
          ev.relateException(new ICAL.Event(ex));
        }
        const add = (
          start: any,
          end: any,
          recurrenceId?: any,
          effective = component,
        ) => {
          const sd = start.toJSDate(),
            ed = end.toJSDate();
          if (ed < rangeStart || sd > rangeEnd) return;
          result.push({
            raw,
            component: effective,
            masterComponent: component,
            uid: ev.uid,
            href: raw.href,
            etag: raw.etag,
            key: raw.href + "#" + start.toString(),
            title: effective.getFirstPropertyValue("summary") || "(No title)",
            start: sd,
            end: ed,
            allDay: start.isDate,
            recurring: ev.isRecurring(),
            recurrenceId: recurrenceId?.toString(),
            originalStart: sd,
            calendar: calendar?.href,
            color: calendar?.color || "#1a73e8",
            task:
              effective.getFirstPropertyValue("x-workspace-task") === "true",
            complete:
              effective.getFirstPropertyValue("x-workspace-completed") ===
              "true",
          });
        };
        if (ev.isRecurring()) {
          const iter = ev.iterator();
          let count = 0,
            next;
          while ((next = iter.next()) && count++ < 10000) {
            if (next.toJSDate() > rangeEnd) break;
            const occ = ev.getOccurrenceDetails(next);
            if (
              occ.item.component.getFirstPropertyValue("status") !== "CANCELLED"
            )
              add(occ.startDate, occ.endDate, next, occ.item.component);
          }
        } else add(ev.startDate, ev.endDate);
      }
    } catch (error) {
      console.warn("A calendar item could not be read", raw.href);
    }
  }
  return result;
}
function emptyForm(
  date = key(new Date()),
  minutes = 9 * 60,
  allDay = false,
  calendar = "",
) {
  const start =
    String(Math.floor(minutes / 60)).padStart(2, "0") +
    ":" +
    String(minutes % 60).padStart(2, "0");
  const end =
    String(Math.floor((minutes + 60) / 60) % 24).padStart(2, "0") +
    ":" +
    String(minutes % 60).padStart(2, "0");
  return {
    uid: crypto.randomUUID(),
    title: "",
    startDate: date,
    endDate: key(addDays(localDate(date), Math.floor((minutes + 60) / 1440))),
    startTime: start,
    endTime: end,
    allDay,
    calendar,
    location: "",
    guests: "",
    description: "",
    recurrence: "none",
    scope: "series",
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    reminder: "10",
    availability: "busy",
    visibility: "default",
    type: "event",
    completed: false,
    meetingUrl: "",
  };
}
const zoneCache = new Map<string, any>();
// Build portable VTIMEZONE data from the browser's IANA rules, including DST.
// Existing imports retain their own timezone components. New rules cover 1970-2100.
function timezoneFor(zone: string) {
  if (zone === "UTC" || zone === "Etc/UTC") return ICAL.Timezone.utcTimezone;
  if (zoneCache.has(zone))
    return new ICAL.Timezone(
      new ICAL.Component(structuredClone(zoneCache.get(zone))),
    );
  const format = new Intl.DateTimeFormat("en-GB", {
    timeZone: zone,
    timeZoneName: "longOffset",
  });
  const offset = (stamp: number) => {
    const name =
      format
        .formatToParts(new Date(stamp))
        .find((p) => p.type === "timeZoneName")?.value || "GMT";
    const m = name.match(/GMT([+-])(\d{1,2}):(\d{2})/);
    return m ? (Number(m[2]) * 60 + Number(m[3])) * (m[1] === "-" ? -1 : 1) : 0;
  };
  const root = new ICAL.Component("vtimezone");
  root.updatePropertyWithValue("tzid", zone);
  root.updatePropertyWithValue("x-lic-location", zone);
  const add = (stamp: number, before: number, after: number) => {
    const c = new ICAL.Component(after > before ? "daylight" : "standard");
    c.updatePropertyWithValue(
      "dtstart",
      ICAL.Time.fromString(
        new Date(stamp + before * 60000).toISOString().slice(0, 19),
      ),
    );
    c.updatePropertyWithValue(
      "tzoffsetfrom",
      ICAL.UtcOffset.fromSeconds(before * 60),
    );
    c.updatePropertyWithValue(
      "tzoffsetto",
      ICAL.UtcOffset.fromSeconds(after * 60),
    );
    root.addSubcomponent(c);
  };
  let previous = Date.UTC(1970, 0, 1),
    old = offset(previous);
  add(previous, old, old);
  const step = 15 * 86400000,
    until = Date.UTC(2101, 0, 1);
  for (let point = previous + step; point <= until + step; point += step) {
    const end = Math.min(point, until),
      next = offset(end);
    if (next !== old) {
      let lo = previous,
        hi = end;
      while (hi - lo > 60000) {
        const mid = Math.floor((lo + hi) / 120000) * 60000;
        if (offset(mid) === old) lo = mid;
        else hi = mid;
      }
      add(hi, old, next);
      old = next;
    }
    previous = end;
  }
  zoneCache.set(zone, root.toJSON());
  return new ICAL.Timezone(root);
}
function zonedTime(date: string, time: string, zone: string) {
  const value = ICAL.Time.fromString(date + "T" + time + ":00");
  value.zone = timezoneFor(zone);
  return value;
}
function encode(form: any, existing: any) {
  let root = existing
    ? new ICAL.Component(ICAL.parse(existing.raw.data))
    : new ICAL.Component(["vcalendar", [], []]);
  if (!existing) {
    root.updatePropertyWithValue("version", "2.0");
    root.updatePropertyWithValue("prodid", "-//One Workspace//Calendar//EN");
  }
  let c: any;
  if (existing?.recurring && form.scope === "event") {
    c = new ICAL.Component(structuredClone(existing.component.toJSON()));
    for (const name of ["rrule", "rdate", "exdate", "recurrence-id"])
      c.removeAllProperties(name);
    const rid = ICAL.Time.fromString(
      existing.recurrenceId ||
        ICAL.Time.fromJSDate(existing.originalStart, true).toString(),
    );
    c.updatePropertyWithValue("recurrence-id", rid);
    const ridZone = existing.component
      .getFirstProperty("dtstart")
      ?.getParameter("tzid");
    if (ridZone)
      c.getFirstProperty("recurrence-id").setParameter("tzid", ridZone);
    const old = root
      .getAllSubcomponents("vevent")
      .find(
        (item: any) =>
          item.getFirstPropertyValue("recurrence-id")?.toString() ===
          rid.toString(),
      );
    if (old) root.removeSubcomponent(old);
    root.addSubcomponent(c);
  } else {
    c = existing
      ? root
          .getAllSubcomponents("vevent")
          .find((c: any) => !c.hasProperty("recurrence-id"))
      : new ICAL.Component("vevent");
    if (!existing) root.addSubcomponent(c);
  }
  if (!existing)
    c.updatePropertyWithValue("uid", form.uid || crypto.randomUUID());
  c.updatePropertyWithValue("dtstamp", ICAL.Time.fromJSDate(new Date(), true));
  let start = form.allDay
    ? ICAL.Time.fromDateString(form.startDate)
    : zonedTime(form.startDate, form.startTime, form.timezone);
  let end = form.allDay
    ? ICAL.Time.fromDateString(key(addDays(localDate(form.endDate), 1)))
    : zonedTime(form.endDate, form.endTime, form.timezone);
  if (!form.allDay && form.timezone !== "UTC" && form.timezone !== "Etc/UTC") {
    const zone = timezoneFor(form.timezone);
    ICAL.TimezoneService.register(zone);
    if (
      !root
        .getAllSubcomponents("vtimezone")
        .some((c: any) => c.getFirstPropertyValue("tzid") === form.timezone)
    )
      root.addSubcomponent(
        new ICAL.Component(structuredClone(zone.component.toJSON())),
      );
  }
  if (end.compare(start) <= 0)
    throw new Error("End time must be after start time.");
  if (existing?.recurring && form.scope === "series") {
    const original = (
      existing.masterComponent || existing.component
    ).getFirstPropertyValue("dtstart");
    const offset = +start.toJSDate() - +existing.originalStart;
    const shifted = new Date(+original.toJSDate() + offset);
    const duration = +end.toJSDate() - +start.toJSDate();
    start = ICAL.Time.fromJSDate(shifted, true);
    end = ICAL.Time.fromJSDate(new Date(+shifted + duration), true);
    if (!form.allDay) {
      start = start.convertToZone(timezoneFor(form.timezone));
      end = end.convertToZone(timezoneFor(form.timezone));
    }
    if (form.allDay) {
      start.isDate = true;
      end.isDate = true;
    }
  }
  c.removeAllProperties("dtstart");
  c.removeAllProperties("dtend");
  c.addPropertyWithValue("dtstart", start);
  c.addPropertyWithValue("dtend", end);
  if (!form.allDay && start.zone !== ICAL.Timezone.utcTimezone) {
    c.getFirstProperty("dtstart").setParameter("tzid", start.zone.tzid);
    c.getFirstProperty("dtend").setParameter("tzid", end.zone.tzid);
  }
  c.removeAllProperties("duration");
  for (const [prop, value] of Object.entries({
    summary: form.title || "(No title)",
    url: form.meetingUrl || "",
    location: form.location,
    description: form.description,
    transp: form.availability === "free" ? "TRANSPARENT" : "OPAQUE",
    class: form.visibility === "default" ? "" : form.visibility.toUpperCase(),
    "x-workspace-task": form.type === "task" ? "true" : "false",
    "x-workspace-appointment": form.type === "appointment" ? "true" : "false",
    status: "CONFIRMED",
    "x-workspace-completed": form.completed ? "true" : "false",
  })) {
    if (prop === "class" && form.visibility === "default")
      c.removeAllProperties("class");
    else c.updatePropertyWithValue(prop, value);
  }
  c.removeAllProperties("attendee");
  for (const address of String(form.guests || "")
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean)) {
    if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(address))
      throw new Error("Enter guest email addresses separated by commas.");
    const p = new ICAL.Property("attendee");
    p.setValue("mailto:" + address);
    p.setParameter("role", "REQ-PARTICIPANT");
    c.addProperty(p);
  }
  if (
    !(existing?.recurring && form.scope === "event") &&
    form.recurrence !== "custom"
  ) {
    c.removeAllProperties("rrule");
    if (form.recurrence !== "none")
      c.addPropertyWithValue(
        "rrule",
        ICAL.Recur.fromString("FREQ=" + form.recurrence),
      );
  }
  c.removeAllSubcomponents("valarm");
  if (form.reminder !== "none") {
    const alarm = new ICAL.Component("valarm");
    alarm.updatePropertyWithValue("action", "DISPLAY");
    alarm.updatePropertyWithValue(
      "description",
      form.title || "Calendar reminder",
    );
    alarm.updatePropertyWithValue(
      "trigger",
      ICAL.Duration.fromSeconds(-Number(form.reminder) * 60),
    );
    c.addSubcomponent(alarm);
  }
  return root.toString() + "\r\n";
}

export {
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
};
