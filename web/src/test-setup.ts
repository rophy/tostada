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

class MockEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  close() {}
}
globalThis.EventSource = MockEventSource as unknown as typeof EventSource
