import { useEffect, useRef, useState } from 'react'
import { Ic } from './Icons'

type Option = readonly [value: string, label: string]

interface Props {
  label: string
  value: string
  options: readonly Option[]
  onChange: (v: string) => void
}

export function FilterChip({ label, value, options, onChange }: Props) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [])

  const sel = options.find((o) => o[0] === value)
  return (
    <div ref={ref} style={{ position: 'relative' }}>
      <button className={`filter ${value !== 'all' ? 'active' : ''}`} onClick={() => setOpen(!open)}>
        <span style={{ color: 'var(--ink-3)' }}>{label}:</span>
        {sel ? sel[1] : value}
        <Ic.chev />
      </button>
      {open && (
        <div
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            marginTop: 4,
            zIndex: 50,
            background: 'var(--panel)',
            border: '1px solid var(--line)',
            borderRadius: 4,
            minWidth: 160,
            padding: 4,
            boxShadow: '0 8px 24px -8px oklch(0% 0 0 / 0.2)',
          }}
        >
          {options.map(([v, l]) => (
            <div
              key={v}
              onClick={() => {
                onChange(v)
                setOpen(false)
              }}
              style={{
                padding: '5px 10px',
                fontSize: 12,
                cursor: 'pointer',
                borderRadius: 3,
                background: v === value ? 'var(--accent-soft)' : 'transparent',
                color: v === value ? 'var(--accent-ink)' : 'var(--ink)',
                fontFamily: 'var(--font-mono)',
              }}
              onMouseEnter={(e) => {
                if (v !== value) (e.currentTarget as HTMLDivElement).style.background = 'var(--bg-3)'
              }}
              onMouseLeave={(e) => {
                if (v !== value) (e.currentTarget as HTMLDivElement).style.background = 'transparent'
              }}
            >
              {l}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
