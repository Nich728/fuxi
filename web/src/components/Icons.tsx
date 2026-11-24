// Line icons drawn for Fuxi: 1.6px strokes, round joins, 24px grid.

type P = { size?: number }

const svg = (size: number, children: React.ReactNode) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.6"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
  >
    {children}
  </svg>
)

export const HomeIcon = ({ size = 24 }: P) => svg(size, <path d="M4 11.5 12 5l8 6.5V19a1 1 0 0 1-1 1h-4.5v-5h-5v5H5a1 1 0 0 1-1-1z" />)

/** A small stack of cards. */
export const DecksIcon = ({ size = 24 }: P) =>
  svg(
    size,
    <>
      <rect x="5" y="8" width="14" height="11" rx="2" />
      <path d="M7.5 5h9" />
    </>,
  )

export const SearchIcon = ({ size = 24 }: P) =>
  svg(
    size,
    <>
      <circle cx="11" cy="11" r="6" />
      <path d="m15.5 15.5 4 4" />
    </>,
  )

/** Three rising seals. */
export const ProgressIcon = ({ size = 24 }: P) =>
  svg(
    size,
    <>
      <rect x="4" y="13" width="4" height="6" rx="1" />
      <rect x="10" y="9" width="4" height="10" rx="1" />
      <rect x="16" y="5" width="4" height="14" rx="1" />
    </>,
  )

export const SettingsIcon = ({ size = 24 }: P) =>
  svg(
    size,
    <>
      <path d="M5 7h9M18 7h1M5 17h1M10 17h9" />
      <circle cx="16" cy="7" r="2" />
      <circle cx="8" cy="17" r="2" />
    </>,
  )

export const CloseIcon = ({ size = 24 }: P) => svg(size, <path d="M6 6l12 12M18 6 6 18" />)

export const BackIcon = ({ size = 24 }: P) => svg(size, <path d="M14.5 5.5 8 12l6.5 6.5" />)

export const PlusIcon = ({ size = 24 }: P) => svg(size, <path d="M12 5v14M5 12h14" />)

export const CheckIcon = ({ size = 24 }: P) => svg(size, <path d="m5 12.5 4.5 4.5L19 7.5" />)

export const ChevronIcon = ({ size = 20 }: P) => svg(size, <path d="m9.5 6 6 6-6 6" />)
