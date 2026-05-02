import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AppDetailDrawer } from './AppDetail'
import type { Application } from '@/lib/types'

// AppDetailDrawer fans out to a lot of useQuery hooks. We mock
// global fetch to return empty bodies for everything; the test only
// cares about the drawer's chrome (dialog semantics, focus
// management, Esc-to-close, tab activation).

const sampleApp: Application = {
  id: 'alpha/shop/Kustomization/checkout',
  name: 'checkout',
  kind: 'Kustomization',
  ns: 'shop',
  cluster: 'Alpha',
  clusterId: 'alpha',
  status: 'healthy',
  sync: 'Synced',
  source: 'flux/podinfo',
  revision: 'abc123',
  age: '5m',
  suspended: false,
}

describe('AppDetailDrawer', () => {
  let qc: QueryClient

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    globalThis.fetch = vi.fn().mockImplementation(async (url: string) => {
      // Return shapes that match each endpoint's response type so
      // hooks don't throw while parsing. Empty arrays/strings keep
      // the rendered components in their empty states.
      const path = String(url)
      let body: unknown = {}
      if (path.includes('/diff')) body = { resources: [] }
      else if (path.includes('/render'))
        body = {
          source: { name: '', namespace: '', kind: '', revision: '', method: '' },
          resources: [],
        }
      else if (path.includes('/tree')) body = { nodes: [] }
      else if (path.includes('/history')) body = { entries: [] }
      else if (path.includes('/manifest')) body = { yaml: '' }
      else if (path.includes('/logs')) body = { pods: [], logs: '' }
      else if (path.includes('/events')) body = { events: [] }
      return {
        ok: true,
        status: 200,
        json: async () => body,
      } as unknown as Response
    }) as unknown as typeof fetch
  })
  afterEach(() => vi.restoreAllMocks())

  function renderDrawer(onClose = vi.fn()) {
    render(
      <QueryClientProvider client={qc}>
        <AppDetailDrawer app={sampleApp} onClose={onClose} />
      </QueryClientProvider>,
    )
    return { onClose }
  }

  it('renders as a dialog with aria-modal and an accessible name', () => {
    renderDrawer()
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(dialog).toHaveAccessibleName(/checkout/i)
  })

  it('focuses the close button on mount', () => {
    renderDrawer()
    const close = screen.getByRole('button', { name: /close application details/i })
    expect(close).toHaveFocus()
  })

  it('calls onClose when Escape is pressed', async () => {
    const { onClose } = renderDrawer()
    const user = userEvent.setup()
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
  })

  it('exposes tabs as a tablist with arrow-key navigation', async () => {
    renderDrawer()
    const tabs = screen.getAllByRole('tab')
    expect(tabs.length).toBeGreaterThan(1)

    // Default selected tab is "Overview".
    const overviewTab = tabs.find((t) => /overview/i.test(t.textContent ?? ''))!
    expect(overviewTab).toHaveAttribute('aria-selected', 'true')

    // Right arrow on the selected tab moves to the next.
    overviewTab.focus()
    const user = userEvent.setup()
    await user.keyboard('{ArrowRight}')

    const treeTab = tabs.find((t) => /resource tree/i.test(t.textContent ?? ''))!
    expect(treeTab).toHaveAttribute('aria-selected', 'true')
  })

  it('shows the segmented mode toggle on the Diff tab', async () => {
    renderDrawer()
    const user = userEvent.setup()
    // Click the Diff tab.
    const diffTab = screen.getByRole('tab', { name: /^diff$/i })
    await user.click(diffTab)

    const modeBar = screen.getByRole('tablist', { name: /diff mode/i })
    const drift = within(modeBar).getByRole('tab', { name: /drift/i })
    const gitVs = within(modeBar).getByRole('tab', { name: /git vs cluster/i })
    expect(drift).toBeInTheDocument()
    expect(gitVs).toBeInTheDocument()
    expect(drift).toHaveAttribute('aria-selected', 'true')
  })
})
