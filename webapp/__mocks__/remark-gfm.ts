/**
 * Jest stub for remark-gfm (ESM-only package).
 * Returns a no-op plugin so tests that pass it to react-markdown do not fail.
 */
export default function remarkGfm() {
  return undefined
}
