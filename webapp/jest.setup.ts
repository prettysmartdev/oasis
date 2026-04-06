import '@testing-library/jest-dom'

// jsdom does not implement scrollTo — polyfill it so HomescreenLayout tests pass
Element.prototype.scrollTo = jest.fn() as typeof Element.prototype.scrollTo
