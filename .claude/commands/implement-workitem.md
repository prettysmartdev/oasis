Implement work item $ARGUMENTS.

First, read:
1. The plan at `aspec/work-items/plans/$ARGUMENTS-*-plan.md`
2. The work item at `aspec/work-items/$ARGUMENTS-*.md` (Implementation Details section)
3. The relevant aspec/ files listed in the plan

Then implement the work item according to the plan. Follow these rules:
- Implement comprehensively — the build must succeed and all existing tests must pass when you are done
- Use the appropriate subagent(s) for each component: **go-backend** for Go code, **frontend** for Next.js/TypeScript, **devops** for Dockerfile/Makefile/CI
- Do NOT write new tests yet (that is the next step)
- Do NOT write or change documentation yet (that comes later)
- Fix any existing tests you break
- Respect all critical invariants in CLAUDE.md
- Follow all conventions in aspec/

After implementation, run the build and verify it succeeds:
- `make build` (Go binaries)
- `npm --prefix webapp run build` (Next.js static export)
- Report any failures and fix them before declaring done.
