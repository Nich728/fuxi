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

export const SearchIcon = ({ size = 24 }: P) =>
  svg(
    size,
    <>
      <circle cx="11" cy="11" r="6" />
      <path d="m15.5 15.5 4 4" />
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

export const ArrowIcon = ({ size = 20 }: P) => svg(size, <path d="M5 12h14M13 6l6 6-6 6" />)
