import '@testing-library/jest-dom/vitest'

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
})

Object.defineProperty(window, 'getComputedStyle', {
  value: () => ({
    getPropertyValue: () => '',
  }),
})

class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver = MockResizeObserver as unknown as typeof ResizeObserver

export class MockEventSource {
  static instances: MockEventSource[] = []
  url: string
  onmessage: ((ev: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  constructor(url: string) {
    this.url = url
    MockEventSource.instances.push(this)
  }
  close() {}
  simulateMessage(data: string) {
    this.onmessage?.({ data } as MessageEvent)
  }
  simulateError() {
    this.onerror?.()
  }
}
globalThis.EventSource = MockEventSource as unknown as typeof EventSource
