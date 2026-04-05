# Work Item: [Feature | Bug | Task]

Title: <short, imperative description, e.g. "Add app enable/disable command to CLI">
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary:
- <1–3 sentences describing what this work item delivers and why it matters>

## User Stories

### User Story 1:
As a: [Owner / Admin | Tailnet Visitor]

I want to:
<what the user wants to do, in plain language>

So I can:
<the outcome or value they get from doing it>

### User Story 2 (if needed):
As a: [Owner / Admin | Tailnet Visitor]

I want to:
<what the user wants to do>

So I can:
<the outcome or value>


## Implementation Details:
- <List specific technical decisions, package locations, API endpoints to add/change, DB schema changes, etc.>
- <Reference relevant aspec/ sections for context (e.g. "see architecture/apis.md for conventions")>
- <Note any third-party packages involved (cobra, crossplane-go, tsnet, etc.)>


## Edge Case Considerations:
- <What happens when the upstream URL is unreachable at registration time?>
- <What happens if the slug already exists?>
- <What if the container is stopped when the CLI command runs?>
- <Add cases specific to this work item>

## Test Considerations:
- <Unit tests: which packages/functions need coverage? What inputs/outputs to validate?>
- <Integration tests: which end-to-end flows should be exercised?>
- <Edge cases from above that must be covered by tests>

## Codebase Integration:
- Follow established conventions, best practices, testing, and architecture patterns from the project's aspec
- Management API changes must follow the REST conventions in architecture/apis.md
- New CLI commands must follow the command/flag structure in uxui/cli.md
- Controller changes must not break graceful NGINX reload or restart safety guarantees
