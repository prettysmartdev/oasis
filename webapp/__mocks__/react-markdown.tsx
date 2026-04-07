/**
 * Jest stub for react-markdown (ESM-only package).
 * Renders children as plain text inside a div so tests can render components
 * that use <ReactMarkdown> without needing the real ESM bundle.
 */
import React from 'react'

interface ReactMarkdownProps {
  children?: React.ReactNode
  remarkPlugins?: unknown[]
}

export default function ReactMarkdown({ children }: ReactMarkdownProps) {
  return <div data-testid="react-markdown">{children}</div>
}
