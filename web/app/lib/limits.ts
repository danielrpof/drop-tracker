// Mirrors tags.MaxNameRunes / tags.MaxTagsPerArtist / watchlist.MaxNoteRunes.
export const MAX_TAG_LENGTH = 32
export const TAG_COUNTER_THRESHOLD = 25
export const MAX_TAGS_PER_ARTIST = 10
export const MAX_NOTE_LENGTH = 500
export const NOTE_WARNING_THRESHOLD = 450

// The server counts Unicode code points, not UTF-16 units.
export function charCount(s: string): number {
  return Array.from(s).length
}

// Mirrors tags.NormalizeName's length basis (NFC, trim, collapsed spaces),
// minus lowercasing, which can change code-point counts.
export function tagNameLength(raw: string): number {
  return charCount(raw.normalize("NFC").trim().replace(/\s+/g, " "))
}

// Mirrors watchlist.NormalizeNote: trimmed, deliberately not NFC.
export function noteLength(raw: string): number {
  return charCount(raw.trim())
}
