import { describe, expect, it } from "vitest"

import {
  formatAbsoluteTime,
  formatClock,
  formatDuration,
  formatIsoTitle,
  formatPollInterval,
  formatRelativeTime,
} from "~/lib/format"

const EM_DASH = "—"

describe("timezone self-check (19-RESEARCH.md Pitfall 4)", () => {
  it("reports the matching UTC hour for a fixed UTC instant under the pinned TZ=UTC runner", () => {
    // vitest.config.ts pins test.env.TZ to "UTC" -- if that pin were ever
    // dropped or ignored by the runner, this is the first case to go red,
    // long before any HH:MM:SS assertion below would silently drift.
    const instant = new Date("2026-01-15T14:03:00Z")
    expect(instant.getHours()).toBe(14)
  })
})

describe("formatRelativeTime", () => {
  const now = new Date("2026-01-15T12:00:00Z")

  it("returns the em dash for null", () => {
    expect(formatRelativeTime(null, now)).toBe(EM_DASH)
  })

  it("clamps a timestamp later than now to 'just now' rather than a future-tense phrase", () => {
    const future = new Date("2026-01-15T12:05:00Z").toISOString()
    expect(formatRelativeTime(future, now)).toBe("just now")
  })

  it.each([
    ["2026-01-15T11:59:20Z", "just now"], // 40s ago, < 45s
    ["2026-01-15T11:59:00Z", "1 minute ago"], // 60s ago, < 90s
    ["2026-01-15T11:55:00Z", "5 minutes ago"], // 5 min ago, < 60m
    ["2026-01-15T11:00:00Z", "1 hour ago"], // 60 min ago, < 90m
    ["2026-01-15T09:00:00Z", "3 hours ago"], // 3h ago, < 24h
    ["2026-01-14T11:00:00Z", "1 day ago"], // 25h ago, >= 24h and < 48h
    ["2026-01-12T12:00:00Z", "3 days ago"], // 3 days ago
  ])("renders %s relative to now as %s", (iso, expected) => {
    expect(formatRelativeTime(iso, now)).toBe(expected)
  })
})

describe("formatAbsoluteTime", () => {
  const now = new Date("2026-01-15T18:00:00Z")

  it("returns the em dash for null", () => {
    expect(formatAbsoluteTime(null, now)).toBe(EM_DASH)
  })

  it("returns the em dash for an unparseable string rather than JavaScript's invalid-date text", () => {
    expect(formatAbsoluteTime("not-a-date", now)).toBe(EM_DASH)
  })

  it("renders 24-hour HH:MM:SS when the timestamp falls on the client's current date", () => {
    expect(formatAbsoluteTime("2026-01-15T09:03:07Z", now)).toBe("09:03:07")
  })

  it("renders MMM D, HH:MM when the timestamp falls on an earlier day", () => {
    expect(formatAbsoluteTime("2026-01-10T09:03:07Z", now)).toBe(
      "Jan 10, 09:03"
    )
  })
})

describe("formatIsoTitle", () => {
  it("includes the full date, time, and a UTC offset marker", () => {
    const title = formatIsoTitle("2026-01-15T09:03:07Z")
    expect(title).toContain("2026")
    expect(title).toContain("09:03:07")
  })

  it("returns the em dash for an unparseable string", () => {
    expect(formatIsoTitle("not-a-date")).toBe(EM_DASH)
  })
})

describe("formatClock", () => {
  it("renders 24-hour HH:MM:SS for an injected Date", () => {
    expect(formatClock(new Date("2026-01-15T23:05:09Z"))).toBe("23:05:09")
  })
})

describe("formatDuration", () => {
  it("returns the em dash for null", () => {
    expect(formatDuration(null)).toBe(EM_DASH)
  })

  it.each([
    [840, "840ms"], // well under the 1000ms bucket edge
    [999, "999ms"], // just under the 1000ms bucket edge
    [1000, "1.0s"], // at the 1000ms bucket edge -- seconds bucket begins
    [3120, "3.1s"], // mid seconds bucket
    [59999, "60.0s"], // just under the 60000ms bucket edge -- still seconds
    [60000, "1m 00s"], // at the 60000ms bucket edge -- minutes bucket begins
    [63000, "1m 03s"], // mid minutes bucket, zero-padded seconds
  ])("renders %ims as %s", (ms, expected) => {
    expect(formatDuration(ms)).toBe(expected)
  })
})

describe("formatPollInterval", () => {
  it.each([
    [900, "15 minutes"], // the fresh-instance default
    [90, "90 seconds"], // not a whole minute multiple
    [1, "1 second"], // singular
    [60, "1 minute"], // singular minute
    [3600, "1 hour"], // whole-hour collapse, singular
    [7200, "2 hours"], // whole-hour collapse, plural
  ])("renders %i seconds as %s", (seconds, expected) => {
    expect(formatPollInterval(seconds)).toBe(expected)
  })
})
