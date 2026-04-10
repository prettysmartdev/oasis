# CLI Design

Binary name: oasis
Install path: /usr/local/bin/oasis or $HOME/.local/bin/
Storage location: $HOME/.oasis/

## Design principles:

### Command structure
Top level command groups:
- oasis init [--advanced]                 — interactive first-time setup: prompts for Tailscale auth key and hostname, pulls the image, starts the container; --advanced unhides the management port prompt
- oasis start                             — start the oasis container if it is stopped
- oasis stop                              — stop the oasis container
- oasis restart                           — restart the oasis container
- oasis status                            — show controller status: Tailscale connection, NGINX state, registered app count, version
- oasis update [--version v1.2.3]         — pull the specified (or latest) image tag and restart the container, preserving volumes
- oasis logs [--follow] [--lines N]       — stream or print controller and NGINX logs from the container
- oasis app new <name>                    — write an app YAML template to ./oasis-app-<name>.yaml; edit the file then use `oasis app add -f` to register
- oasis app add --name <n> --url <u> --slug <s> [--description <d>] [--icon <emoji|url>] [--tags <t,...>] [--access-type direct|proxy]
- oasis app add -f/--file <path>          — register an app from a YAML definition file; if -f is provided alongside other flags, flags are ignored with a warning
- oasis app list [--json]                 — list all registered apps with name, slug, upstream URL, status, and health
- oasis app show <slug> [--json]          — show full details for a single app
- oasis app remove <slug>                 — unregister and remove an app
- oasis app enable <slug>                 — enable a disabled app (adds it back to the dashboard and NGINX routes)
- oasis app disable <slug>               — disable an app (hides it from the dashboard and removes its NGINX route, but keeps the record)
- oasis app update <slug> [--name] [--url] [--description] [--icon] [--tags] [--access-type direct|proxy] — update app fields
- oasis agent new <name>                  — write an agent YAML template to ./oasis-agent-<name>.yaml; edit the file then use `oasis agent add -f` to register
- oasis agent add [--name <n>] [--slug <s>] [--prompt <p>] [--trigger tap|schedule|webhook] [--schedule <cron>] [--output-fmt markdown|html|plaintext] [--model <model-id>] [--description <d>] [--icon <emoji|url>] [-f/--file <path>]
- oasis agent list [--json]               — list all registered agents with name, slug, trigger, enabled state
- oasis agent show <slug> [--json]        — show full details for a single agent, including last run status
- oasis agent remove <slug>               — remove agent and all its run history
- oasis agent enable <slug>               — enable a disabled agent (scheduler resumes firing it; webhook accepts requests)
- oasis agent disable <slug>              — disable an agent (scheduler skips it; webhook returns 409)
- oasis agent update <slug> [--name] [--prompt] [--trigger] [--schedule] [--output-fmt] [--model] [--description] [--icon] — update agent fields
- oasis settings get [key]               — print current settings (or a single key's value)
- oasis settings set <key> <value>       — update a settings value
- oasis db backup [--output <path>] [--db-path <container-path>] — download a copy of the SQLite database to the host; --db-path overrides the default /data/db/oasis.db for non-standard installs

### Flag structure
Flag guidance:
- Global flags (available on all commands): --config <path> (CLI config file, default ~/.oasis/config.json), --json (output machine-readable JSON instead of human-readable text), --quiet (suppress non-error output), --version (print CLI version and exit)
- Command-specific flags are documented in each command's --help output
- Follow POSIX conventions: short single-char flags (-j) and long flags (--json); boolean flags do not take a value
- Flags can be placed before or after subcommands (cobra handles this)
- Mutually exclusive flags produce a clear error message rather than silently ignoring one

### Inputs and outputs
I/O Guidance:
- stdin: used only for interactive prompts during `oasis init`; all other commands are fully non-interactive by design (suitable for scripts)
- stdout: human-readable, table-formatted text by default; clean JSON arrays/objects when --json is passed
- stderr: all error messages, warnings, and progress indicators; exit code 0 on success, 1 on error, 2 on usage error
- Suppress ANSI color codes and spinner animations when stdout/stderr is not a TTY (auto-detect via isatty)
- Progress spinners shown during slow operations (image pull, Tailscale connection wait) when outputting to a TTY

### Configuration
Global config ($HOME/.oasis/config.json):
- mgmtEndpoint: full URL of the management API (default: "http://127.0.0.1:04515") — override if running on a non-standard port
- containerName: Docker container name to manage (default: "oasis")
- lastKnownVersion: last image version the CLI successfully ran — used for update checks and version skew warnings
- The config file is created and managed by `oasis init`; users rarely need to edit it manually
