import { CoverArt } from "~/components/common/CoverArt"
import type { WatchlistEntry } from "~/lib/api"

export interface WatchlistRowProps {
  entry: WatchlistEntry
}

// WatchlistRow renders one WatchlistEntry: the artist's art, name, and --
// only when present -- their disambiguation text. D-14: image_url flows
// through the shared CoverArt component, which already supplies the
// null-art placeholder. disambiguation is omitted entirely when null --
// never an empty parenthetical or stray separator, since both strings
// originate from MusicBrainz/Deezer and are rendered as plain JSX text
// nodes -- React's default auto-escaping, the T-06-12 mitigation this
// component relies on. A name too long for the row's name column truncates
// with an ellipsis and carries the full name via a native title attribute.
//
// The preference-toggle and remove-control slots below are filled by this
// plan's task 2 (PreferenceToggles.tsx and the remove handler wired from
// the route).
export function WatchlistRow({ entry }: WatchlistRowProps) {
  return (
    <li className="flex flex-col gap-4 rounded-md bg-card p-4 sm:flex-row sm:items-center">
      <CoverArt src={entry.image_url} alt={entry.name} size={64} />

      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <span
          className="truncate text-heading font-semibold text-foreground"
          title={entry.name}
        >
          {entry.name}
        </span>
        {entry.disambiguation !== null && (
          <span className="truncate text-label text-muted-foreground">
            {entry.disambiguation}
          </span>
        )}
      </div>

      {/* Preference toggles slot -- filled by task 2's PreferenceToggles */}
      {/* Remove control slot -- filled by task 2's remove handler */}
    </li>
  )
}
