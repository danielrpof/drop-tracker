import { CoverArt } from "~/components/common/CoverArt"
import { Badge } from "~/components/ui/badge"
import type { EventItem } from "~/lib/api"

// EventCard dispatches on event.event_type to one of three bodies defined
// below in this same file (D-08): all three share the hero CoverArt, the
// event title, the artist name, and a small color badge -- the only place
// the three event-type colors from app.css are used (06-UI-SPEC.md Color:
// the indigo accent stays reserved for primary buttons/nav/focus rings and
// is never applied to a badge).
export interface EventCardProps {
  event: EventItem
}

// EVENT_BADGE maps each event_type to its badge copy, emoji and color token
// -- reused verbatim from Phase 5's Discord embed colors (D-01/D-08).
const EVENT_BADGE: Record<
  EventItem["event_type"],
  { label: string; emoji: string; color: string }
> = {
  new_release: {
    label: "New release",
    emoji: "🆕",
    color: "var(--color-event-new-release)",
  },
  guest_feature: {
    label: "Guest feature",
    emoji: "🎤",
    color: "var(--color-event-guest-feature)",
  },
  deluxe_change: {
    label: "Deluxe change",
    emoji: "💿",
    color: "var(--color-event-deluxe-change)",
  },
}

// UNKNOWN_EVENT_BADGE covers an event_type value outside EVENT_BADGE's keys.
// GET /events returns the event type unvalidated, so the Record type above
// is a compile-time claim the network boundary does not enforce -- one
// unrecognized row must not be able to take the whole History route down
// through the top-level error boundary.
const UNKNOWN_EVENT_BADGE = {
  label: "Unknown",
  emoji: "❔",
  color: "var(--color-muted-foreground)",
}

export function EventCard({ event }: EventCardProps) {
  // Same rationale as UNKNOWN_EVENT_BADGE above: the lookup falls back
  // rather than dereferencing undefined for an out-of-union event_type.
  const badge = EVENT_BADGE[event.event_type] ?? UNKNOWN_EVENT_BADGE

  return (
    <div className="flex gap-4 rounded-2xl bg-card p-4 ring-1 ring-foreground/10">
      <CoverArt
        src={event.cover_art_url}
        alt={event.title}
        size={96}
        className="rounded-md"
      />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <div className="flex items-start justify-between gap-3">
          {/* Long titles truncate to ~2 lines with an ellipsis; the full
              title is still available via the native title tooltip. */}
          <h3
            className="line-clamp-2 text-heading font-semibold text-foreground"
            title={event.title}
          >
            {event.title}
          </h3>
          <Badge
            variant="secondary"
            className="shrink-0 gap-1 text-background"
            style={{ backgroundColor: badge.color }}
          >
            <span aria-hidden="true">{badge.emoji}</span>
            {badge.label}
          </Badge>
        </div>
        <p className="text-body text-muted-foreground">{event.artist_name}</p>
        <EventCardBody event={event} />
      </div>
    </div>
  )
}

// EventCardBody dispatches to the type-specific body -- new_release,
// guest_feature and deluxe_change each render different detail fields,
// mirroring the distinct Discord embed shapes from Phase 5 (D-08).
function EventCardBody({ event }: { event: EventItem }) {
  switch (event.event_type) {
    case "new_release":
      return <NewReleaseBody event={event} />
    case "guest_feature":
      return <GuestFeatureBody event={event} />
    case "deluxe_change":
      return <DeluxeChangeBody event={event} />
    default:
      return null
  }
}

// NewReleaseBody shows the release type and date -- a null release_date
// renders "Release date unknown" rather than a blank line (D-14 partial
// state).
function NewReleaseBody({ event }: { event: EventItem }) {
  const dateLabel = event.release_date ?? "Release date unknown"
  return (
    <p className="text-label text-muted-foreground">
      {event.release_type ? `${event.release_type} · ` : ""}
      {dateLabel}
    </p>
  )
}

// guestFeatureHref builds a link from the event's own identifier fields
// (source + external_id) rather than passing a stored URL string straight
// into an anchor -- this endpoint never stores a raw URL for guest_feature
// rows, only the source-specific external id. external_id is escaped with
// encodeURIComponent before interpolation: it is an unvalidated third-party
// string typed only as `string`, so today's UUID-shaped values are a fact
// about the current upstreams, not a guarantee the code can rely on.
function guestFeatureHref(event: EventItem): string | null {
  if (event.source === "musicbrainz") {
    return `https://musicbrainz.org/recording/${encodeURIComponent(event.external_id)}`
  }
  if (event.source === "deezer") {
    return `https://www.deezer.com/track/${encodeURIComponent(event.external_id)}`
  }
  return null
}

// GuestFeatureBody shows the recording title (event.title is the recording
// title for a guest_feature row) linked out to the source.
function GuestFeatureBody({ event }: { event: EventItem }) {
  const href = guestFeatureHref(event)
  if (!href) {
    return <p className="text-label text-muted-foreground">{event.title}</p>
  }
  return (
    <p className="text-label text-muted-foreground">
      Featured on{" "}
      <a
        href={href}
        target="_blank"
        rel="noreferrer"
        className="underline underline-offset-2 hover:text-foreground"
      >
        {event.title}
      </a>
    </p>
  )
}

// DeluxeChangeBody shows the track-count delta as previous → current
// tracks. A null previous_track_count is defensive-only (D-04 should
// always populate it) and renders the current track_count alone, no arrow.
function DeluxeChangeBody({ event }: { event: EventItem }) {
  const current = event.track_count ?? "?"
  if (event.previous_track_count == null) {
    return <p className="text-label text-muted-foreground">{current} tracks</p>
  }
  return (
    <p className="text-label text-muted-foreground">
      {event.previous_track_count} → {current} tracks
    </p>
  )
}
