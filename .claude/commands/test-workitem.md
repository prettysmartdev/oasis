Write tests for work item $ARGUMENTS.

First, read:
1. The work item at `aspec/work-items/$ARGUMENTS-*.md` (Test Considerations section)
2. The plan at `aspec/work-items/plans/$ARGUMENTS-*-plan.md`
3. `aspec/foundation.md` (best practices for testing)

Then implement all tests described in the Test Considerations section. Follow these rules:

**Go tests:**
- One test file per package (`*_test.go`)
- Use `go test -race ./...` — all tests must pass with the race detector
- Test exported behaviour via inputs/outputs, not internals
- `TestControllerStartup` — server starts, returns 501 on unknown route
- `TestRootCmd` — `--version` prints non-empty string, exits 0
- Management API loopback binding must be asserted in tests
- Coverage threshold: start low (10%) to avoid blocking CI on skeleton code

**Web tests:**
- Jest + `@testing-library/react`
- One smoke test per page/component
- `npm test -- --ci` must pass

After writing tests, run `make test` and verify all pass. Report any failures and fix them.
