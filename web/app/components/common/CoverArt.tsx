import { Music } from "lucide-react"
import { useEffect, useState } from "react"

import { cn } from "~/lib/utils"

// CoverArt renders a hero-sized image from a nullable URL (D-14: "large,
// hero-style, art-forward"). When src is null/undefined, or the image
// fails to load, it renders a styled placeholder built from a lucide
// music-note icon on the secondary surface token instead of a broken
// image -- D-14's "graceful fallback for null art". Shared by the History
// feed (cover_art_url) and the Watchlist rows (image_url), so props stay
// generic rather than event/artist-specific.
export interface CoverArtProps {
  src?: string | null
  alt: string
  size?: number
  className?: string
}

export function CoverArt({ src, alt, size = 96, className }: CoverArtProps) {
  const [failed, setFailed] = useState(false)

  // D-01: reset the failed flag whenever src changes on this retained
  // instance -- WatchlistRow, EventCard and SearchResultsColumns all reuse
  // the same CoverArt instance across re-renders rather than remounting via
  // key={src}. Choosing an effect-based reset over a key-based remount is a
  // deliberate, locked deviation from React's own "prefer key remount"
  // guidance (react.dev/learn/you-might-not-need-an-effect), made to avoid
  // touching those three call sites. The accepted cost is one render frame
  // that can still show the stale placeholder before this effect commits
  // (12-RESEARCH.md Pitfall 1). Do not revert this to a remount-based reset.
  useEffect(() => {
    setFailed(false)
  }, [src])

  const showPlaceholder = !src || failed

  const style = { width: size, height: size }

  if (showPlaceholder) {
    return (
      <div
        role="img"
        aria-label={alt}
        className={cn(
          "flex shrink-0 items-center justify-center rounded-md bg-secondary text-muted-foreground",
          className
        )}
        style={style}
      >
        <Music className="h-1/3 w-1/3" aria-hidden="true" />
      </div>
    )
  }

  return (
    <img
      src={src}
      alt={alt}
      style={style}
      className={cn("shrink-0 rounded-md object-cover", className)}
      onError={() => setFailed(true)}
    />
  )
}
