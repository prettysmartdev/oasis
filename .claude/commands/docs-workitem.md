Write documentation for work item $ARGUMENTS.

First, read:
1. The work item at `aspec/work-items/$ARGUMENTS-*.md`
2. The plan at `aspec/work-items/plans/$ARGUMENTS-*-plan.md`
3. `aspec/devops/localdev.md` (documentation standards)

Then write documentation following these rules:
- Add or update godoc-style package comments on all exported Go types and functions introduced in this work item
- Update `README.md` if installation steps, quickstart, or CLI usage changed
- Update relevant `aspec/` files if any architectural decisions were made during implementation that differ from the spec (aspec is a living document)
- Update `aspec/architecture/apis.md` if any new endpoints were added
- Update `aspec/uxui/cli.md` if any new CLI commands were added
- Update `.env.local.example` if any new environment variables were introduced
- Do NOT add comments to code that is self-evident — only where logic is non-obvious

Do not modify the work item file itself.
