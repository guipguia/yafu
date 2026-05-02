interface Props {
  data: number[]
  status?: 'ok' | 'warn' | 'err'
  width?: number
  height?: number
}

export function Sparkline({ data, status = 'ok', width = 240, height = 32 }: Props) {
  if (data.length < 2) return null
  const min = Math.min(...data)
  const max = Math.max(...data)
  const range = max - min || 1
  const w = width
  const h = height
  const step = w / (data.length - 1)
  const pts = data.map((v, i): [number, number] => [
    i * step,
    h - 4 - ((v - min) / range) * (h - 8),
  ])
  const line = pts.map((p, i) => (i === 0 ? `M${p[0]},${p[1]}` : `L${p[0]},${p[1]}`)).join(' ')
  const area = `${line} L${w},${h} L0,${h} Z`
  return (
    <svg className={`spark ${status}`} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <path className="area" d={area} />
      <path className="line" d={line} />
    </svg>
  )
}
