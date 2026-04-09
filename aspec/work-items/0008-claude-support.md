# Work Item: Feature

Title: claude support — real agent execution via the claude CLI
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary

Replace the stub agent agentharness from WI-0005 with a real implementation backed by the `claude` CLI, making agents in Oasis actually run AI workloads. Introduce a persistent chat interface in the webapp powered by a direct claude call (no file-based output pipeline), so users can interact with Claude conversationally from any page on the homescreen. This is the first of several planned AI backends; the implementation is structured around a Go `AgentHarness` interface so future backends (e.g. Gemini, local Ollama) can be added without changes to the run lifecycle.

---

## User Stories

### User Story 1: Chat bar conversation
As a: Tailnet Visitor

I want to:
tap the text box in the oasis chat bar to open a texting-style chat overlay, type a message, and get a response from Claude — with the full conversation history persisted so it is there the next time I open it

So I can:
ask Claude questions, get help with tasks, and refer back to previous exchanges without losing context

### User Story 2: Agent execution with real AI output
As a: Owner / Admin

I want to:
register an agent with a prompt, trigger it (via tap, schedule, or webhook), and see real AI-generated output — rendered as markdown, HTML, or plain text — in the agent window, written by Claude into a run-specific directory on the persistent volume

So I can:
automate recurring AI tasks (daily digests, summaries, file generation) and view their output directly in the dashboard without any manual steps

### User Story 3: Per-agent model selection
As a: Owner / Admin

I want to:
optionally set a `model` field on an agent definition (e.g. `claude-opus-4-6`) so that specific agents use a particular Claude model, while agents without a model field use claude's default

So I can:
optimise cost and capability per agent — lighter models for routine tasks, more capable models for complex ones

---

## Implementation Details

### 1. AgentHarness interface (`internal/controller/agent/agentharness.go`)

Refactor the existing stub `Run` function into a Go interface. The interface is the contract all AI backends must satisfy.

```go
// Package agent manages the agent run lifecycle and defines the AgentHarness interface
// that all AI backend implementations must satisfy.
package agent

// AgentHarness runs an agent's prompt in a pre-created work directory.
// Implementations are expected to write their output to a file in workDir
// as instructed by the system prompt; the caller reads that file after Execute returns.
type AgentHarness interface {
    // Execute runs the agent and returns when the process has finished or ctx is cancelled.
    // workDir is the absolute path to a pre-created directory dedicated to this run.
    // The AgentHarness must set the subprocess CWD to workDir.
    Execute(ctx context.Context, a db.Agent, workDir string) error
}
```

Remove the old stub `Run` function entirely. Rename the file to `agentharness.go` if it was `agentharness.go` already. Update all call sites in the scheduler and HTTP handler to use `AgentHarness.Execute`.

### 2. Claude agentharness (`internal/controller/agent/claude/`)

New package `internal/controller/agent/claude/` with a single exported type `ClaudeHarness` that implements `agent.AgentHarness`.

**Binary flags (always present):**
- `--print` — non-interactive mode; required on every invocation
- `--permission-mode acceptEdits` — allow the claude CLI to write files without prompting
- `--system-prompt <generated>` — see system prompt template below

**Binary flags (conditional):**
- `--model <value>` — only included when `agent.Model` is non-empty

**User prompt:** passed as the last positional argument (not a flag). No stdin piping needed.

**CWD:** `exec.Cmd.Dir` must be set to `workDir` so the claude CLI's relative path references resolve inside the run directory.

**System prompt template** (use `text/template` to render):

```
You are an AI assistant integrated into the Oasis homescreen. Your task is:

{{.Prompt}}

Output format: {{.OutputFmt}}

Write your complete response to the file: {{.OutputFile}}
Do not output anything else. The file you write must be valid {{.OutputFmt}}. The file must exist when you finish.
```

Where:
- `Prompt` = `agent.Prompt`
- `OutputFmt` = `agent.OutputFmt` (e.g. "markdown")
- `OutputFile` = `filepath.Join(workDir, outputFilename(agent.OutputFmt))` — `result.md` for markdown, `result.html` for html, `result.txt` for plaintext

**After `Execute` returns nil error:** the caller (run handler) reads `OutputFile`, stores its contents in `agent_runs.output`, then updates `status = "done"`. If `OutputFile` does not exist after a successful exit, treat as `status = "error"` with message `"agent produced no output file"`.

**On non-zero exit code:** write the combined stdout+stderr to `error.txt` inside the workDir, and then save it to `agent_runs.output`; return an error so the run is marked `status = "error"`. The user should be able to see what error the agent output when they attempt to view the agent run that failed in the webapp.

**Constructor:**

```go
// New returns a ClaudeAgentHarness that invokes the claude binary at the given path.
// Pass an empty string to resolve "claude" via PATH at call time.
func New(binaryPath string) *AgentHarness
```

**Env var `OASIS_CLAUDE_BIN`** — override the claude binary path (useful for testing). Default: resolve `claude` from `$PATH`.

### 3. Agent run work directory

**Env var `OASIS_AGENT_RUNS_DIR`** — base directory for all run work dirs. Default: `/data/agent-runs`. Lives on the same Docker volume as `/data/db/` and `/data/ts-state/`.

**Directory lifecycle:**
1. Before creating the `AgentRun` DB row, call `os.MkdirAll(filepath.Join(runsDir, runID), 0o750)`.
2. Pass the full path to `AgentHarness.Execute`. Buffer stdin/stderr to write into error.txt if the agent returns an error or if the agent exits and no result file exists.
3. After `Execute` returns, read the output file or error.txt and store its contents in `agent_runs.output`.
4. Do not delete the work directory — it is retained on the persistent volume for debugging. Future work items may add a cleanup policy.

Add `OASIS_AGENT_RUNS_DIR` to `.env.local.example` and the env vars table in `CLAUDE.md`.

### 4. Agent `model` field — data model and API

**SQLite migration 4** (`internal/controller/db/store.go`):

```sql
ALTER TABLE agents ADD COLUMN model TEXT NOT NULL DEFAULT '';

CREATE TABLE chat_messages (
    id          TEXT PRIMARY KEY,   -- UUID v4
    role        TEXT NOT NULL,      -- "user" | "assistant"
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL       -- RFC3339
);

PRAGMA user_version = 4;
```

**Go type update (`internal/controller/db/agents.go`):**

Add `Model string \`json:"model"\`` to the `Agent` struct. Update `CreateAgent`, `scanAgent`, and `UpdateAgent` to include the new column.

**API changes:**
- `POST /api/v1/agents` — accept optional `"model"` string field; no validation beyond non-empty when present (the claude agentharness passes it verbatim to `--model`)
- `PATCH /api/v1/agents/:slug` — allow updating `model`
- `GET /api/v1/agents` / `GET /api/v1/agents/:slug` — include `model` in the response

**YAML agent definition** — add optional `model` field:

```yaml
model: ""   # Claude model ID, e.g. "claude-opus-4-6". Omit to use the default model.
```

Update `ParseAgentFile` and the `oasis agent new` template to include this field.

**CLI:** add `--model` flag (string, optional, default `""`) to `oasis agent add` and `oasis agent update`.

Update the `Agent` object definition in `aspec/architecture/apis.md` to include `model (string, optional)`.

### 5. Chat messages — data model

**Go type (`internal/controller/db/chat.go`):**

```go
// ChatMessage represents a single turn in the persistent chat history.
type ChatMessage struct {
    ID        string    `json:"id"`
    Role      string    `json:"role"`      // "user" | "assistant"
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"createdAt"`
}
```

**Store interface additions (`internal/controller/db/store.go`):**
- `CreateChatMessage(ctx, ChatMessage) error`
- `ListChatMessages(ctx) ([]ChatMessage, error)` — ordered by `created_at ASC`

### 6. Chat API endpoints

Chat endpoints are served on **both** the management API and the tsnet API (chat is a user-facing feature, not an admin-only one).

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/chat/messages` | Send a message; runs claude; returns user + assistant messages |
| GET  | `/api/v1/chat/messages` | Return full chat history; `{ "items": [...], "total": N }` |

**`POST /api/v1/chat/messages` request body:**
```json
{ "message": "What is the capital of France?" }
```

**`POST /api/v1/chat/messages` response (200):**
```json
{
  "userMessage":      { "id": "...", "role": "user",      "content": "...", "createdAt": "..." },
  "assistantMessage": { "id": "...", "role": "assistant", "content": "...", "createdAt": "..." }
}
```

**Chat run logic** (synchronous, unlike agent runs which are async):
1. Validate `message` is non-empty; 400 if blank.
2. Persist the user `ChatMessage` row immediately.
3. Run `claude --print <message>` (no `--system-prompt`, no work dir, CWD can be `/tmp`); capture stdout as the response.
4. Persist the assistant `ChatMessage` row.
5. Return both rows in the response.

The chat path does **not** create an agent run directory or use the `AgentHarness` interface — it is a lightweight direct invocation. Use a reasonable timeout (configurable via `OASIS_CHAT_TIMEOUT`, default 120s).

Add `OASIS_CHAT_TIMEOUT` to `.env.local.example`.

Add `ChatMessage` to the Objects section of `aspec/architecture/apis.md`:
```
ChatMessage: { id (uuid), role ("user"|"assistant"), content (string), createdAt (RFC3339) }
```

### 7. Claude authentication setup (`oasis init` additions)

This section extends the existing `oasis init` interactive flow with Claude auth setup steps. All changes are in `internal/cli/root.go` (or wherever `oasis init` is implemented).

#### Step A — Detect claude config on host

Before starting the container, check whether the user already has a local Claude installation:

```go
home, _ := os.UserHomeDir()
claudeJSON := filepath.Join(home, ".claude.json")
claudeDir  := filepath.Join(home, ".claude")
hasClaudeJSON := fileExists(claudeJSON)
hasClaudeDir  := dirExists(claudeDir)
```

Store both booleans for use in steps C and D below.

#### Step B — Offer Claude authentication

After collecting the Tailscale auth key and hostname (but before starting the container), prompt:

```
Configure Claude authentication now? [Y/n]
```

Default is yes. If the user declines, skip to step C with an empty token.

If the user accepts:
1. Check whether `claude` is available on the host's `$PATH` via `exec.LookPath("claude")`.
   - If not found, print a warning to stderr and skip to step C with an empty token:
     ```
     Warning: 'claude' binary not found on PATH. Skipping Claude authentication setup.
     You can configure this later by re-running 'oasis init' or setting the token manually.
     ```
2. If found, exec `claude setup-token` with the subprocess's stdin/stdout/stderr connected to the terminal so the user sees the full interactive flow.
3. After `claude setup-token` exits (any exit code — it may exit non-zero on some platforms while still displaying the token), prompt:
   ```
   Paste the OAuth token you copied:
   ```
4. Read the token from stdin (trim all surrounding whitespace). If blank, warn and continue with an empty token:
   ```
   Warning: No token provided. Claude features will be unavailable until a token is configured.
   ```

#### Step C — Send init request with token

Include the token (empty string if skipped) in the same `POST /api/v1/setup` request body:

```json
{
  "ts_auth_key":        "tskey-...",
  "hostname":           "oasis",
  "claude_oauth_token": "ey..."
}
```

`claude_oauth_token` is **optional**; an empty string means no token. The controller must accept either. This field must **never be logged** by the CLI (same treatment as `ts_auth_key`).

#### Step D — Docker run volume mounts

When `oasis init` builds the `docker run` command to start the container:

- If BOTH `~/.claude.json` AND `~/.claude/` exist on the host, add these bind-mount flags to the `docker run` invocation:
  ```
  -v "$HOME/.claude.json:/root/.claude.json:ro"
  -v "$HOME/.claude/:/root/.claude/:ro"
  ```
  The CLI must expand `~` / use the real home directory path (no shell tilde expansion in `exec.Command`). These mounts are read-only (`:ro`) — the container should not modify the host's claude state.

- If EITHER or BOTH paths are absent, print a post-init warning to stderr after the container starts:
  ```
  Warning: ~/.claude.json and/or ~/.claude/ were not found on this host.
  Claude features inside oasis require authentication. To complete setup, run:

    docker exec -it oasis claude
  ```

#### Step E — Controller: in-memory token storage

Add a `claudeOAuthToken string` field to the main controller struct (`internal/controller/`). Populate it when `POST /api/v1/setup` is called with a non-empty `claude_oauth_token`:

```go
type Controller struct {
    // ...existing fields...
    claudeOAuthToken string // never persisted, never logged
}
```

Rules:
- **Never write this value to SQLite** — hold it only in the controller struct.
- **Never log this value** — same treatment as `TS_AUTHKEY` (critical invariant #4 applies here too).
- **Never return it in any API response.**
- If the controller restarts, the token is lost; the user must re-run `oasis init` (or a future `oasis settings set claude-token` command — out of scope for this work item). Document this limitation in a code comment.

#### Step F — Inject token into all claude subprocesses

Extend the `ClaudeHarness` constructor to accept additional environment variables:

```go
// New returns a ClaudeAgentHarness that invokes the claude binary at the given path.
// Pass an empty binaryPath to resolve "claude" via PATH at call time.
// extraEnv entries (e.g. "CLAUDE_CODE_OAUTH_TOKEN=...") are injected into every subprocess.
func New(binaryPath string, extraEnv []string) *AgentHarness
```

When building `exec.Cmd` inside `Execute`:

```go
cmd.Env = append(os.Environ(), h.extraEnv...)
```

The controller passes `[]string{"CLAUDE_CODE_OAUTH_TOKEN=" + token}` when the token is non-empty, or `nil` when not set.

Apply the same env injection to the chat direct-invocation path (section 6): when constructing the `exec.Cmd` for `claude --print <message>`, append `CLAUDE_CODE_OAUTH_TOKEN=<token>` to the environment if the token is set.

The controller provides a helper method to avoid token-injection logic being scattered across callers:

```go
// claudeEnv returns the extra env vars to inject into claude subprocesses.
// Returns nil if no token has been configured.
func (c *Controller) claudeEnv() []string {
    if c.claudeOAuthToken == "" {
        return nil
    }
    return []string{"CLAUDE_CODE_OAUTH_TOKEN=" + c.claudeOAuthToken}
}
```

Update `aspec/architecture/apis.md` — the `POST /api/v1/setup` description to note the new `claude_oauth_token` optional field.

---

### 8. Dockerfile — install claude CLI

In the final image stage (`debian:bookworm-slim`), install Node.js and the `@anthropic-ai/claude-code` npm package globally:

```dockerfile
# Install the Claude CLI
RUN curl -fsSL https://claude.ai/install.sh | bash \
    && cp /root/.local/bin/claude /usr/local/bin/claude
```

Verify the installed binary is reachable at `/usr/local/bin/claude` and on `$PATH` via a build-time `RUN claude --version` smoke test.

### 9. Webapp — chat overlay

#### `BottomNav` changes (`webapp/components/BottomNav.tsx`)

The palm tree button already reveals the chat bar. **Do not auto-focus the text input when the bar slides open** — remove any `autoFocus` or `ref.current.focus()` call triggered by the palm tree toggle. The text box must remain visually present but unfocused until the user explicitly taps it.

Add an `onChatOpen` prop (called when the text input receives focus, i.e. when the user taps the text box). This triggers the chat overlay to open.

When the chat overlay is open, hide the chat text input in the bar (the overlay has its own input). Show only the palm tree icon as a dismiss affordance (pressing it while chat is open closes the overlay and returns the bar to its collapsed, non-focused state).

#### New component: `ChatOverlay` (`webapp/components/ChatOverlay.tsx`)

A full-screen overlay (slides up from below, `z-50`) rendered on top of all other content including the proxy app iFrame view.

**Layout:**
- Header: "Chat" label + X close button (top-right). Pressing X closes the overlay; the chat bar returns to its non-focused state (text input is blurred).
- Message thread: scrollable `div` with `overflowY: auto`, grows to fill available space. Messages rendered in a texting-style bubble layout — user messages right-aligned (teal background, `bg-teal-500`), assistant messages left-aligned (slate background, `bg-slate-700`). Each bubble shows a timestamp (`createdAt`) in small text below.
- Input area: fixed at the bottom of the overlay, above the iOS/Android software keyboard. Implement keyboard avoidance using the CSS environment variable `env(keyboard-inset-height, 0px)` combined with `position: fixed; bottom: calc(env(keyboard-inset-height, 0px) + env(safe-area-inset-bottom, 0px))`. Use the Visual Viewport API (`window.visualViewport.height`) as a fallback for browsers that do not support `keyboard-inset-height`.
- Send button: teal, disabled while a response is in flight. Show a spinner on the send button while waiting.
- Empty state: "No messages yet. Say hello!" centred in the thread area.

**Data flow:**
1. On mount, `GET /api/v1/chat/messages` and render the full history. Scroll to the bottom.
2. On send: optimistically append the user's message bubble (teal outline, not filled in to show not yet sent), call `POST /api/v1/chat/messages`, then make the user bubble solid teal and append the assistant bubble when the response arrives. On error, show a red toast and remove the optimistic user bubble, returning their message text to the text box.
3. After each new bubble (optimistic or otherwise) is added, scroll the thread to the bottom.

**Reduced motion:** the slide-up entry animation must be disabled when `prefers-reduced-motion` is set.

#### Webapp API additions (`webapp/lib/api.ts`)

Add:
- `getChatHistory()` — GET `/api/v1/chat/messages` → `{ items: ChatMessage[], total: number }`
- `sendChatMessage(message: string)` — POST `/api/v1/chat/messages` → `{ userMessage: ChatMessage, assistantMessage: ChatMessage }`

Add `ChatMessage` TypeScript interface:
```ts
interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  createdAt: string
}
```

---

## Edge Case Considerations

- **`claude` not on host PATH during `oasis init`** — if `exec.LookPath("claude")` fails during the auth setup step, skip `claude setup-token` with a clear warning. Do not abort `oasis init`; the rest of initialization proceeds normally.
- **`claude setup-token` exits non-zero** — treat any exit code as "maybe succeeded"; always proceed to the token-paste prompt regardless. The user may have already copied the token before the subprocess exited.
- **Token not provided / blank paste** — continue `oasis init` with an empty token. The controller will start without Claude auth; claude subprocesses will rely on whatever ambient auth the container has (e.g. from mounted `~/.claude.json`).
- **Container restart loses the in-memory token** — the `claudeOAuthToken` field is not persisted. On container restart, claude subprocesses will fall back to ambient auth from mounted config files. If no mounted config and no token, claude will fail. This is a known limitation; document it in a comment near the field declaration.
- **`~/.claude.json` exists but `~/.claude/` does not (or vice versa)** — treat as "incomplete config"; skip the bind mounts entirely and show the exec-into-container warning. Both paths must be present to add the mounts.
- **Host home directory not determinable** — if `os.UserHomeDir()` returns an error during `oasis init`, skip the claude config detection step and skip the bind mounts. Log a warning.
- **`claude` binary not found at startup** — the controller must check for the binary on startup and log a warning (not a fatal error) if it is absent. Agent runs triggered while the binary is missing must fail immediately with `status = "error"` and message `"claude binary not found; ensure @anthropic-ai/claude-code is installed"`. The chat endpoint must return 503 with `"code": "EXECUTOR_UNAVAILABLE"`.
- **Agent run output file missing after successful exit** — claude exited 0 but did not write the expected output file. Mark the run as `status = "error"` with message `"agent produced no output file"`. Log the last N bytes of stdout/stderr at debug level for diagnosis.
- **Work directory creation failure** — if `os.MkdirAll` fails (e.g. permissions, disk full), the run must not start; create the `AgentRun` row with `status = "error"` immediately and return the error to the caller.
- **Agent run exceeds timeout** — claude processes are cancelled via context cancellation. Add `OASIS_AGENT_RUN_TIMEOUT` env var (default: 5 minutes). On timeout, mark the run as `status = "error"` with message `"agent run timed out"`.
- **Claude exits non-zero** — capture both stdout and stderr; include the first 512 bytes of combined output in the `agent_runs` error state (store in `output` column prefixed with `"[error] "`). Do not expose raw output to the webapp unless the run status is `"error"`.
- **Chat endpoint timeout** — if claude does not respond within `OASIS_CHAT_TIMEOUT`, return 504 with `"code": "CHAT_TIMEOUT"`. The already-persisted user message is retained; no assistant message is stored.
- **Concurrent chat requests** — the chat endpoint is synchronous; concurrent requests are served independently (no serialisation required). Each call runs its own claude process. Multiple in-flight chat calls are acceptable.
- **Large output file** — no size limit is enforced in this work item. The output column in SQLite can hold large blobs. Document as a known limitation: extremely large outputs (> 10 MB) may cause memory pressure during file read. Add a `OASIS_AGENT_MAX_OUTPUT_BYTES` env var (default: 10 MB) to cap how much of the output file is read; truncate with a notice appended if exceeded.
- **Model field contains an invalid model ID** — the controller passes the value verbatim to `--model`; claude will return an error if the model is invalid, which propagates as a run error. No server-side model validation is performed (avoid hard-coding a model list that goes stale).
- **System prompt template rendering failure** — if the Go template fails to render (should only happen if the template itself is broken), the run must fail fast with a clear error rather than calling claude with a malformed `--system-prompt`.
- **Agent deleted while its run is executing** — the goroutine holds the run ID; when it attempts to call `UpdateAgentRun` after the agent is deleted (cascade deletes the run row), the update fails silently (log at debug). Do not panic.
- **Persistent volume not mounted** — if `/data/agent-runs` is not writable (e.g. Docker volume not mounted), work directory creation fails; see "Work directory creation failure" above. Log a warning at startup if the directory cannot be created.
- **Chat history ordering** — `ListChatMessages` must order by `created_at ASC` to ensure correct conversational order even if IDs are not monotonic.
- **Palm tree re-tap while chat is open** — pressing the palm tree button while the chat overlay is open should close the overlay (same as pressing X), returning the bar to its idle state without blurring/refocusing.

---

## Test Considerations

### Unit tests

**`internal/cli` (`oasis init` — Claude auth setup)**
- When `~/.claude.json` AND `~/.claude/` both exist, the generated `docker run` command includes both `-v` bind-mount flags with `:ro`.
- When either path is absent, the bind-mount flags are omitted and a warning is printed to stderr.
- When the user accepts Claude auth setup and `claude` is on PATH, `claude setup-token` is exec'd with the terminal connected.
- When `claude` is not on PATH, a warning is printed and the token-paste prompt is skipped.
- When the user pastes a non-empty token, the `POST /api/v1/setup` body includes `claude_oauth_token`.
- When the user pastes nothing (blank), the `POST /api/v1/setup` body omits or sends an empty `claude_oauth_token`; no error is raised.
- When the user declines Claude auth, the token is omitted from the init request.
- Token value must not appear in any stdout or stderr output produced by the CLI.

**`internal/controller/api` (setup handler)**
- `POST /api/v1/setup` with `claude_oauth_token` stores the value in the controller's in-memory field.
- `POST /api/v1/setup` without `claude_oauth_token` (or empty) leaves the field empty.
- The token value is not included in any API response, even the setup response.

**`internal/controller` (claudeEnv)**
- `claudeEnv()` returns `[]string{"CLAUDE_CODE_OAUTH_TOKEN=<token>"}` when token is set.
- `claudeEnv()` returns `nil` when token is empty.

**`internal/controller/agent/claude`**
- `Execute` with non-nil `extraEnv` appends those entries to `cmd.Env`.
- `Execute` with nil `extraEnv` does not set `cmd.Env` (inherits `os.Environ()`).
- When `CLAUDE_CODE_OAUTH_TOKEN` is in `extraEnv`, it is present in the subprocess environment.

**`internal/controller/agent/claude`**
- `Execute` invokes the claude binary with `--print` and `--permission-mode acceptEdits` on every call.
- `Execute` includes `--model <value>` only when `agent.Model` is non-empty.
- `Execute` includes `--system-prompt` containing the rendered output file path and the agent prompt.
- `Execute` sets `cmd.Dir` to `workDir`.
- `Execute` returns an error when the claude binary exits non-zero.
- System prompt template renders the correct `OutputFile` path for each of the three `outputFmt` values (`result.md`, `result.html`, `result.txt`).
- `New("")` resolves the binary via `PATH`; `New("/custom/path/claude")` uses the given path.

Use a fake claude binary (a small shell script that writes `"# ok"` to the expected output file and exits 0, or exits 1 to simulate failure) during tests. Set `OASIS_CLAUDE_BIN` to point at the fake binary.

**`internal/controller/db` (chat)**
- `CreateChatMessage` persists all fields.
- `ListChatMessages` returns messages ordered by `created_at ASC`.
- Round-trip: create two user messages and one assistant message; list returns all three in insertion order.

**`internal/controller/api` (chat handlers)**
- `POST /api/v1/chat/messages` with valid body returns 200 with `userMessage` and `assistantMessage`.
- `POST /api/v1/chat/messages` with empty `message` returns 400.
- `POST /api/v1/chat/messages` when claude binary is unavailable returns 503 with `EXECUTOR_UNAVAILABLE`.
- `GET /api/v1/chat/messages` returns messages in chronological order.
- `GET /api/v1/chat/messages` with no history returns `{ "items": [], "total": 0 }`.

**`internal/controller/db` (agent model field)**
- `CreateAgent` with non-empty `Model` persists the value.
- `CreateAgent` with empty `Model` stores an empty string (not NULL).
- `UpdateAgent` via `model` field changes the value; subsequent `GetAgent` returns the new value.

**`internal/controller/agent` (run lifecycle)**
- Work directory is created before `AgentHarness.Execute` is called.
- If `os.MkdirAll` fails, the run is persisted with `status = "error"` and `Execute` is never called.
- If `Execute` returns nil but the output file is absent, run status is set to `"error"`.
- If the output file exists and is non-empty, its contents are stored in `agent_runs.output` and `status = "done"`.
- Run cancelled by context (timeout) results in `status = "error"` with `"timed out"` message.

**`internal/cli/yaml`**
- `ParseAgentFile` parses a `model` field correctly.
- `ParseAgentFile` with `model` omitted defaults to empty string (no error).

**`internal/cli` (agent commands)**
- `oasis agent add --model claude-opus-4-6` sends `model: "claude-opus-4-6"` in the POST body.
- `oasis agent add` without `--model` sends `model: ""` (or omits it, relying on server default).
- `oasis agent update <slug> --model claude-haiku-4-5-20251001` sends a PATCH with `model`.

### Integration tests (`//go:build integration`)

- Full agent run lifecycle with a real (or fake) claude binary: register agent → trigger run → poll until `status = "done"` → verify `output` is non-empty and output file exists on disk in `<OASIS_AGENT_RUNS_DIR>/<runId>/`.
- Chat lifecycle: `POST /api/v1/chat/messages` → verify both messages stored → `GET /api/v1/chat/messages` → verify order and content.
- Timeout: set `OASIS_AGENT_RUN_TIMEOUT=1s`, trigger a slow agent (fake binary that sleeps 5s), verify run ends with `status = "error"` and timeout message.

### Frontend tests

- `ChatOverlay` renders history returned by `getChatHistory`.
- `ChatOverlay` appends a user bubble optimistically on send.
- `ChatOverlay` appends the assistant bubble after `sendChatMessage` resolves.
- `ChatOverlay` removes the optimistic bubble and shows an error toast when `sendChatMessage` rejects.
- `ChatOverlay` scrolls to the bottom after each new message.
- `BottomNav` text input does not receive focus when the palm tree button is tapped (no `autoFocus`).
- `BottomNav` calls `onChatOpen` when the text input receives focus.
- `ChatOverlay` X button calls `onClose`.
- Reduced motion: `ChatOverlay` entry animation is skipped when `prefers-reduced-motion` is set.

---

## Codebase Integration

- Follow established conventions, best practices, testing, and architecture patterns from the project's aspec.
- The `AgentHarness` interface must live in `internal/controller/agent/agentharness.go`. The claude implementation lives in `internal/controller/agent/claude/`. All exported types and functions must carry godoc-style package and inline comments (critical invariant #9).
- The `claude` package must use `os/exec` to launch the binary; do not shell out via `sh -c` (avoids shell injection if prompt content is passed as an argument).
- Migration 4 must use the existing `PRAGMA user_version` guard pattern in `internal/controller/db/store.go`. Check `version < 4`. Never use `version == 3`.
- Chat and agent run endpoints must not break the loopback-only constraint for the management API (critical invariant #1). Chat endpoints are served on both servers — ensure the tsnet server registration includes the chat routes.
- `OASIS_AGENT_RUNS_DIR`, `OASIS_AGENT_RUN_TIMEOUT`, `OASIS_CHAT_TIMEOUT`, and `OASIS_CLAUDE_BIN` must all be added to `.env.local.example` with documented defaults and to the env vars table in `CLAUDE.md`.
- The Dockerfile `npm install -g @anthropic-ai/claude-code` must pin a specific version. Add a comment noting the version and where to update it for releases.
- The `claude` binary is run as uid 1000 (non-root container, critical invariant #3); ensure the `agent-runs` volume directory is created with permissions that uid 1000 can write to (mode `0o750` is sufficient if owned by uid 1000).
- Do not add NGINX proxy routes for agents or chat — agents are run-and-display, not upstream-proxy resources (same constraint as WI-0005).
- Add `ChatMessage` to the Objects section of `aspec/architecture/apis.md`. Add `model` to the `Agent` object definition. Add chat endpoints to the Endpoint Summary section.
- Add `model` to the agent YAML fields block in `aspec/architecture/apis.md`.
- The `claudeOAuthToken` field on the controller struct must carry a godoc comment reading: `// claudeOAuthToken is the Claude OAuth token provided at setup. It is held in memory only — never persisted and never logged. Lost on restart.`
- `POST /api/v1/setup` must be updated in `aspec/architecture/apis.md` to document the new optional `claude_oauth_token` field.
- `claude_oauth_token` must never appear in controller logs. Add a lint comment or a log-sanitisation test if the project has one.
- The `New(binaryPath string, extraEnv []string)` signature change for `ClaudeHarness` must be reflected in all call sites (agent scheduler, chat handler). No other packages should construct `ClaudeHarness` directly.
- `oasis init` must expand `~` to the real home directory using `os.UserHomeDir()` before checking for `.claude.json` / `.claude/` and before building the `-v` flags — never rely on shell tilde expansion.
- The bind-mount paths in the `docker run` command are `:ro` (read-only). The container user (uid 1000) must not be expected to write to these mounts.
- Run `golangci-lint run ./internal/controller/... ./internal/cli/... ./cmd/...` before marking done; fix all reported issues.
- Run `npm run lint` and `tsc --noEmit` in `webapp/` before marking done.
