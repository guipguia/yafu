export class APIError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'APIError'
  }
}

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...init?.headers,
    },
  })

  if (res.status === 401) {
    handleUnauthorized()
    // Return a never-resolving promise so React Query doesn't render an
    // error state while window.location.assign navigates away.
    return new Promise<T>(() => {})
  }

  if (!res.ok) {
    let detail = ''
    try {
      const body = (await res.json()) as { error?: string }
      if (body?.error) detail = `: ${body.error}`
    } catch {
      // body not JSON; ignore
    }
    throw new APIError(res.status, `HTTP ${res.status}${detail}`)
  }
  return (await res.json()) as T
}

// handleUnauthorized redirects the browser to /auth/login with a return
// path so the user lands back on the page they tried to access. In auth
// modes that don't expose /auth/login (anonymous / header), the request
// will surface a 404 — visible enough to prompt operator investigation.
function handleUnauthorized(): void {
  if (typeof window === 'undefined') return

  // Avoid redirect loops if /auth/login itself triggered a 401, or if
  // we're already on a static error page.
  if (window.location.pathname.startsWith('/auth/')) return

  const ret = encodeURIComponent(window.location.pathname + window.location.search)
  window.location.assign(`/auth/login?return=${ret}`)
}
