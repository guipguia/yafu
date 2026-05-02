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
