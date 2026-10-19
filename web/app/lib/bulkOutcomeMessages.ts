import type { BulkAddTagOutcome, BulkRemoveTagOutcome } from "~/lib/api"
import { formatAlreadyRemoved, formatNameList } from "~/lib/format"

// Pure toast copy for bulk outcomes; the caller owns sonner and any action.

export interface ToastSpec {
  level: "success" | "warning" | "error"
  title: string
  description?: string
  duration?: number
}

const LONG_MS = 8000

export function plural(n: number): string {
  return n === 1 ? "" : "s"
}

function countGone(skipped: readonly { reason: string }[]): number {
  return skipped.filter((s) => s.reason === "gone").length
}

function withDescription(
  level: ToastSpec["level"],
  title: string,
  gone: number,
  extra: Partial<ToastSpec> = {}
): ToastSpec {
  return {
    level,
    title,
    ...(gone > 0 && { description: formatAlreadyRemoved(gone) }),
    ...extra,
  }
}

export function failureToast(
  maxIds: number | null,
  fallback: string
): ToastSpec {
  return {
    level: "error",
    title:
      maxIds === null
        ? fallback
        : `Too many artists selected — select at most ${maxIds} and try again.`,
  }
}

// Cap skips are named in `sentIds` order; `nameOf` covers skips the server left unnamed.
export function addTagToast(
  outcome: BulkAddTagOutcome,
  sentIds: readonly number[],
  nameOf: (id: number) => string,
  variant: "add" | "restore" = "add"
): ToastSpec {
  const n = outcome.applied.length
  const tag = `“${outcome.tag.name}”`
  const caps = outcome.skipped
    .filter((s) => s.reason === "cap")
    .sort((a, b) => sentIds.indexOf(a.id) - sentIds.indexOf(b.id))
  const names = formatNameList(caps.map((s) => s.name ?? nameOf(s.id)))
  const gone = countGone(outcome.skipped)
  const join = (...parts: string[]) => parts.filter(Boolean).join(" ")
  const goneNote = gone > 0 ? formatAlreadyRemoved(gone) : ""

  if (caps.length > 0 && n === 0) {
    return {
      level: "warning",
      title:
        variant === "restore"
          ? `Couldn't restore ${tag} — every artist already has 10 tags.`
          : `Couldn't add ${tag} — every selected artist already has 10 tags.`,
      description: join(`${names}.`, goneNote),
      duration: LONG_MS,
    }
  }
  const title =
    variant === "restore"
      ? `Restored ${tag} on ${n} artist${plural(n)}.`
      : `Added ${tag} to ${n} artist${plural(n)}.`
  if (caps.length > 0) {
    const k = caps.length
    return {
      level: "warning",
      title,
      description: join(
        `${k} already ${k === 1 ? "has" : "have"} 10 tags: ${names}.`,
        goneNote
      ),
      duration: LONG_MS,
    }
  }
  return withDescription("success", title, gone)
}

export function removeTagToast(
  tagName: string,
  outcome: BulkRemoveTagOutcome
): ToastSpec {
  return withDescription(
    "success",
    `Removed “${tagName}” from ${outcome.carried} artist${plural(outcome.carried)}.`,
    countGone(outcome.skipped),
    { duration: LONG_MS }
  )
}

export function preferencesToast(
  applied: number,
  skipped: readonly { reason: string }[]
): ToastSpec {
  return withDescription(
    "success",
    `Updated preferences for ${applied} artist${plural(applied)}.`,
    countGone(skipped)
  )
}

export function removeWatchlistToast(
  total: number,
  alreadyGone: number
): ToastSpec {
  return withDescription(
    "success",
    `Removed ${total} artist${plural(total)} from your watchlist.`,
    alreadyGone
  )
}
