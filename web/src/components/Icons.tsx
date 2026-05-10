import type { SVGProps } from 'react'

type IconProps = SVGProps<SVGSVGElement>

const stroke = {
  fill: 'none' as const,
  stroke: 'currentColor',
  strokeWidth: 1.4,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

export const Ic = {
  dash: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <rect x="2" y="2" width="5" height="6" />
      <rect x="9" y="2" width="5" height="3" />
      <rect x="2" y="10" width="5" height="4" />
      <rect x="9" y="7" width="5" height="7" />
    </svg>
  ),
  cluster: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="8" cy="8" r="2" />
      <circle cx="3" cy="3" r="1.4" />
      <circle cx="13" cy="3" r="1.4" />
      <circle cx="3" cy="13" r="1.4" />
      <circle cx="13" cy="13" r="1.4" />
      <path d="M4 4l2.5 2.5M12 4l-2.5 2.5M4 12l2.5-2.5M12 12l-2.5-2.5" />
    </svg>
  ),
  app: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <rect x="2" y="2" width="5" height="5" />
      <rect x="9" y="2" width="5" height="5" />
      <rect x="2" y="9" width="5" height="5" />
      <rect x="9" y="9" width="5" height="5" />
    </svg>
  ),
  source: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="4" cy="4" r="1.6" />
      <circle cx="4" cy="12" r="1.6" />
      <circle cx="12" cy="8" r="1.6" />
      <path d="M4 5.6v4.8M5.4 12h2.5a2 2 0 0 0 2-2V8.5" />
    </svg>
  ),
  alert: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M8 2c-2.5 0-4 1.8-4 4.2V9l-1.2 2.2A.5.5 0 0 0 3.2 12h9.6a.5.5 0 0 0 .4-.8L12 9V6.2C12 3.8 10.5 2 8 2z" />
      <path d="M6.5 13.5c.3.6.9 1 1.5 1s1.2-.4 1.5-1" />
    </svg>
  ),
  image: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <rect x="2.5" y="3" width="11" height="10" rx="1" />
      <path d="M2.5 10l3-3 3 3 2-2 3 3" />
    </svg>
  ),
  events: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="8" cy="8" r="6" />
      <path d="M8 4.5V8l2 2" />
    </svg>
  ),
  settings: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="8" cy="8" r="2" />
      <path d="M13 8a5 5 0 0 0-.1-1l1.2-.9-1.5-2.6-1.4.5a5 5 0 0 0-1.7-1L9.3 1.5h-3l-.2 1.5a5 5 0 0 0-1.7 1L3 3.5 1.5 6.1l1.2 1A5 5 0 0 0 2.6 8a5 5 0 0 0 .1 1L1.5 9.9 3 12.5l1.4-.5a5 5 0 0 0 1.7 1l.2 1.5h3l.2-1.5a5 5 0 0 0 1.7-1l1.4.5 1.5-2.6-1.2-.9c0-.3.1-.6.1-1z" />
    </svg>
  ),
  chev: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M5 7l3 3 3-3" />
    </svg>
  ),
  search: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="7" cy="7" r="4.5" />
      <path d="M10.5 10.5L13.5 13.5" />
    </svg>
  ),
  refresh: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M13 7A5 5 0 0 0 4 5.5M3 9a5 5 0 0 0 9 1.5M13 3v3.5h-3.5M3 13V9.5h3.5" />
    </svg>
  ),
  pause: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <rect x="4.5" y="3" width="2.5" height="10" />
      <rect x="9" y="3" width="2.5" height="10" />
    </svg>
  ),
  play: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" {...p}>
      <path d="M5 3l8 5-8 5z" />
    </svg>
  ),
  more: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" {...p}>
      <circle cx="3.5" cy="8" r="1.3" />
      <circle cx="8" cy="8" r="1.3" />
      <circle cx="12.5" cy="8" r="1.3" />
    </svg>
  ),
  filter: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M2 3.5h12M4 8h8M6.5 12.5h3" />
    </svg>
  ),
  sun: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="8" cy="8" r="2.5" />
      <path d="M8 1.5v2M8 12.5v2M1.5 8h2M12.5 8h2M3.3 3.3l1.4 1.4M11.3 11.3l1.4 1.4M3.3 12.7l1.4-1.4M11.3 4.7l1.4-1.4" />
    </svg>
  ),
  moon: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M13 9.5A6 6 0 0 1 6.5 3a6 6 0 1 0 6.5 6.5z" />
    </svg>
  ),
  bell: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M8 2c-2.2 0-4 1.6-4 4v2.5L3 11h10l-1-2.5V6c0-2.4-1.8-4-4-4z" />
      <path d="M6.5 12.5c.3.7.9 1 1.5 1s1.2-.3 1.5-1" />
    </svg>
  ),
  warn: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M8 2.5l6 11h-12z" />
      <path d="M8 7v3M8 12v.5" />
    </svg>
  ),
  check: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} strokeWidth={1.6} {...p}>
      <path d="M3 8.5l3.5 3 7-7" />
    </svg>
  ),
  x: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} strokeWidth={1.6} {...p}>
      <path d="M4 4l8 8M12 4l-8 8" />
    </svg>
  ),
  spark: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M8 1.5l1.6 4.4 4.4 1.6-4.4 1.6L8 13.5 6.4 9.1 2 7.5l4.4-1.6z" />
    </svg>
  ),
  graph: (p: IconProps = {}) => (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="3.5" cy="3.5" r="1.6" />
      <circle cx="3.5" cy="12.5" r="1.6" />
      <circle cx="12.5" cy="8" r="1.6" />
      <path d="M5 4l5.7 3.4M5 12l5.7-3.4" />
    </svg>
  ),
  git: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="4" cy="4" r="1.6" />
      <circle cx="4" cy="12" r="1.6" />
      <circle cx="12" cy="8" r="1.6" />
      <path d="M4 5.5v5M5.4 12H8a2 2 0 0 0 2-2V9.5" />
    </svg>
  ),
  helm: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <circle cx="8" cy="8" r="3" />
      <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.2 3.2l1.4 1.4M11.4 11.4l1.4 1.4M3.2 12.8l1.4-1.4M11.4 4.6l1.4-1.4" />
    </svg>
  ),
  oci: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M8 2L2.5 5v6L8 14l5.5-3V5z" />
      <path d="M2.5 5L8 8l5.5-3M8 8v6" />
    </svg>
  ),
  ai: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <path d="M8 1.5l1.5 4 4 1.5-4 1.5L8 12.5 6.5 8.5 2.5 7l4-1.5z" />
      <path d="M13 11.5l.5 1.5.5-1.5L15 11l-1-.5z" />
    </svg>
  ),
  copy: (p: IconProps = {}) => (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} {...p}>
      <rect x="5" y="5" width="9" height="9" rx="1" />
      <path d="M3 11V3a1 1 0 0 1 1-1h7" />
    </svg>
  ),
}
