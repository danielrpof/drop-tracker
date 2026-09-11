// format.ts is the single home for every derived-display formatter the
// System view (Phase 19, D-09/D-09-a, SYS-01/SYS-02) needs. Pure, React-free,
// native Date/Intl only -- CONTEXT locks this phase to zero new dependencies,
// so no date-fns/dayjs/luxon. Every formatter that reads "now" takes it as an
// injected parameter defaulting to the current time, so tests stay
// deterministic without mocking globals (mirrors history.test.tsx's
// controlled-inputs discipline). Six formatters, six call sites:
//
//   - formatRelativeTime  -> per-source panel "Last clean run" line (D-08),
//     the one relative-primary timestamp in the view (D-09 exception)
//   - formatAbsoluteTime  -> every other <time> element's visible value
//     (last-run line, history table rows) -- absolute-primary per D-09
//   - formatIsoTitle      -> those same <time> elements' `title` attribute
//   - formatClock         -> the page-chrome "as of HH:MM:SS" freshness
//     stamp (D-13), taken from the client clock at fetch resolution
//   - formatDuration      -> per-source panel + history table Duration
//     column (SYS-01) -- added by Task 2
//   - formatPollInterval  -> About block "Every {…}" row + first-run copy
//     (SYS-02, D-06) -- added by Task 2

const EM_DASH = "—"

function isValidDate(d: Date): boolean {
  return !Number.isNaN(d.getTime())
}

function sameLocalDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  )
}

const clockFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hourCycle: "h23", // not hour12:false -- some ICU builds render midnight as "24:00:00" under hour12:false
})

const hourMinuteFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
})

const monthDayFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
})

const fullDateFormatter = new Intl.DateTimeFormat(undefined, {
  year: "numeric",
  month: "short",
  day: "numeric",
})

// formatRelativeTime renders the D-08 "Last clean run" line. null -> em
// dash. Any negative delta (client clock behind the server, container
// drift) clamps to "just now" rather than a future-tense phrase (D-09
// clock-skew clamp) -- that clamp and the "< 45s" bucket share one branch
// below since both resolve to the same string.
export function formatRelativeTime(
  iso: string | null,
  now: Date = new Date()
): string {
  if (iso === null) return EM_DASH
  const then = new Date(iso)
  if (!isValidDate(then)) return EM_DASH

  const deltaSec = (now.getTime() - then.getTime()) / 1000
  if (deltaSec < 45) return "just now"
  if (deltaSec < 90) return "1 minute ago"

  const minutes = Math.round(deltaSec / 60)
  if (minutes < 60) return `${minutes} minutes ago`
  if (deltaSec < 90 * 60) return "1 hour ago"

  const hours = Math.round(deltaSec / 3600)
  if (hours < 24) return `${hours} hours ago`
  if (deltaSec < 48 * 3600) return "1 day ago"

  const days = Math.round(deltaSec / 86400)
  return `${days} days ago`
}

// formatAbsoluteTime is the D-09 absolute-primary value: null or an
// unparseable string both degrade to the em dash (never blank, never
// JS's invalid-date text -- D-09 operator-trust line). Same local calendar
// day as `now` -> 24h HH:MM:SS; an earlier day -> "MMM D, HH:MM" (D-09-a) --
// the <=50-entry history buffer can span days on a slow-cadence source, so a
// bare HH:MM in a lower row would be ambiguous.
export function formatAbsoluteTime(
  iso: string | null,
  now: Date = new Date()
): string {
  if (iso === null) return EM_DASH
  const then = new Date(iso)
  if (!isValidDate(then)) return EM_DASH

  if (sameLocalDay(then, now)) {
    return clockFormatter.format(then)
  }
  return `${monthDayFormatter.format(then)}, ${hourMinuteFormatter.format(then)}`
}

// formatIsoTitle is the full local timestamp (with UTC offset) for a
// <time title="…">, so hovering a row resolves both the exact instant and
// the zone it's being read in. Malformed input degrades to the em dash,
// same as formatAbsoluteTime, rather than leaking "Invalid Date".
export function formatIsoTitle(iso: string): string {
  const d = new Date(iso)
  if (!isValidDate(d)) return EM_DASH

  const offsetParts = new Intl.DateTimeFormat(undefined, {
    timeZoneName: "shortOffset",
  }).formatToParts(d)
  const offset = offsetParts.find((p) => p.type === "timeZoneName")?.value ?? ""

  return `${fullDateFormatter.format(d)}, ${clockFormatter.format(d)} ${offset}`.trim()
}

// formatClock is the D-13 "as of" freshness stamp: 24h HH:MM:SS for an
// injected Date taken from the client clock at fetch resolution, never from
// a payload field.
export function formatClock(d: Date): string {
  return clockFormatter.format(d)
}
