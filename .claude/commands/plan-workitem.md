Read work item $ARGUMENTS from `aspec/work-items/` (find the file matching the work item number, e.g. `0001-*.md`). Also read all relevant aspec/ files referenced in the work item and the Codebase Integration section.

Produce a detailed, step-by-step implementation plan. The plan must:
- Break implementation into discrete, ordered steps
- Identify which files to create or modify
- Call out which subagent(s) (go-backend, frontend, devops) handle each step
- List all edge cases from the work item's Edge Case Considerations section
- Note all test requirements from the Test Considerations section
- Flag any ambiguities or open questions before work begins

Do NOT write any code. Write the plan to `aspec/work-items/plans/$ARGUMENTS-{name}-plan.md` where `{name}` is derived from the work item title.
