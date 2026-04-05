Review the implementation of work item $ARGUMENTS for correctness, completeness, security, and style.

First, read:
1. The work item at `aspec/work-items/$ARGUMENTS-*.md` (all sections, especially Edge Case Considerations and Test Considerations)
2. The plan at `aspec/work-items/plans/$ARGUMENTS-*-plan.md`
3. `CLAUDE.md` — critical invariants
4. `aspec/architecture/security.md`

Then review the changes made for this work item:

**Check correctness:**
- Does the implementation match the work item's Implementation Details?
- Do all edge cases from Edge Case Considerations have handling?

**Check security:**
- Management API bound only to `127.0.0.1`?
- `TS_AUTHKEY` never logged or returned?
- No 0.0.0.0 bindings anywhere?
- `.env.local` in `.gitignore`?

**Check build:**
- `CGO_ENABLED=0` enforced?
- Static binary verified?
- All Makefile targets work?

**Check tests:**
- All tests from Test Considerations implemented?
- `go test -race ./...` passes?
- `npm test -- --ci` passes?
- Coverage threshold met?

**Check style:**
- Conventional commits on all changes?
- Godoc comments on all exported symbols?
- No over-engineering or unnecessary abstractions?

Suggest improvements if needed, but **ask before changing anything**.

When complete, provide a short manual test plan so the user can verify the work item end-to-end.
