import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Sidebar } from './Sidebar'
import type { PageId } from './Sidebar'

// Sidebar reads useClusters and useApplications. The test mocks
// global fetch so the hooks return empty data without hitting the
// network.

describe('Sidebar', () => {
  let qc: QueryClient

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
      const body = url.includes('clusters')
        ? { clusters: [] }
        : url.includes('applications')
          ? { applications: [] }
          : {}
      return {
        ok: true,
        status: 200,
        json: async () => body,
      } as unknown as Response
    }) as unknown as typeof fetch
  })
  afterEach(() => vi.restoreAllMocks())

  function renderSidebar(active: PageId = 'fleet', onNav = vi.fn()) {
    render(
      <QueryClientProvider client={qc}>
        <Sidebar active={active} onNav={onNav} side="labeled" cluster={null} />
      </QueryClientProvider>,
    )
    return { onNav }
  }

  it('marks the active item with aria-current="page"', () => {
    renderSidebar('apps')
    const apps = screen.getByText('Applications').closest('a')
    expect(apps).toHaveAttribute('aria-current', 'page')
    const fleet = screen.getByText('Fleet').closest('a')
    expect(fleet).not.toHaveAttribute('aria-current')
  })

  it('calls onNav with the clicked page id', async () => {
    const { onNav } = renderSidebar('fleet')
    const user = userEvent.setup()
    await user.click(screen.getByText('Sources'))
    expect(onNav).toHaveBeenCalledWith('sources')
  })

  it('exposes the navigation as a labeled landmark', () => {
    renderSidebar()
    expect(screen.getByRole('navigation', { name: /main/i })).toBeInTheDocument()
  })

  it('renders an aria-label on icon-only mode for screen readers', () => {
    render(
      <QueryClientProvider client={qc}>
        <Sidebar active="fleet" onNav={vi.fn()} side="icons" cluster={null} />
      </QueryClientProvider>,
    )
    // In "icons" mode the visible label is hidden via CSS, but the
    // anchor still carries aria-label so screen readers can read it.
    expect(screen.getByLabelText('Sources')).toBeInTheDocument()
  })
})
