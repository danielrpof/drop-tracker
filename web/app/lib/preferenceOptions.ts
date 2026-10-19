// Shared by the card's PreferenceToggles and the bulk Preferences popover.
// Type order mirrors ReleaseTypes / EventTypes in internal/watchlist/service.go.

export interface PreferenceOption {
  type: string
  label: string
  aria: string
}

export const RELEASE_OPTIONS: readonly PreferenceOption[] = [
  { type: "album", label: "Album", aria: "Album release type" },
  { type: "single", label: "Single", aria: "Single release type" },
  { type: "ep", label: "EP", aria: "EP release type" },
  { type: "deluxe", label: "Deluxe", aria: "Deluxe release type" },
]

export const MUTE_OPTIONS: readonly PreferenceOption[] = [
  {
    type: "new_release",
    label: "New release",
    aria: "Mute new release alerts",
  },
  {
    type: "guest_feature",
    label: "Guest feature",
    aria: "Mute guest feature alerts",
  },
  {
    type: "deluxe_change",
    label: "Deluxe change",
    aria: "Mute deluxe change alerts",
  },
]

export const RELEASE_TYPE_ORDER = RELEASE_OPTIONS.map((o) => o.type)
export const EVENT_TYPE_ORDER = MUTE_OPTIONS.map((o) => o.type)
