/**
 * The console's mark: the eget archive-and-chip symbol from assets/logo, drawn
 * inline so it inherits currentColor and stays sharp at rail size.
 */
export default function BrandMark({ size = 18 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 512 512"
      role="img"
      aria-label="eget"
      focusable="false"
    >
      <path
        fill="currentColor"
        fillRule="evenodd"
        d="M136 76H280A60 60 0 0 1 340 136V216A124 124 0 0 0 216 340H136A60 60 0 0 1 76 280V136A60 60 0 0 1 136 76Z"
      />
      <rect x="300" y="300" width="136" height="136" rx="34" fill="currentColor" />
    </svg>
  )
}
