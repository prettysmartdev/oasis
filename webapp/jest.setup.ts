import '@testing-library/jest-dom'

// jsdom does not implement scrollTo — polyfill it so HomescreenLayout tests pass
Element.prototype.scrollTo = jest.fn() as typeof Element.prototype.scrollTo

// jsdom does not implement matchMedia — polyfill it so components using
// window.matchMedia('(prefers-reduced-motion: reduce)') don't throw
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: jest.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: jest.fn(),
    removeListener: jest.fn(),
    addEventListener: jest.fn(),
    removeEventListener: jest.fn(),
    dispatchEvent: jest.fn(),
  })),
})
