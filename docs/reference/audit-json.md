# Audit JSON

`csaw audit --json` emits a stable machine-readable report for local checks and CI.

```bash
csaw audit --strict --json
```

Default mode exits nonzero when any finding has severity `error`. `--strict` also exits nonzero when any finding has severity `warn`.

## Report Shape

```json
{
  "project_root": "/path/to/project",
  "policy_path": "/path/to/project/.csaw/policy.yml",
  "policy_found": true,
  "mounted": 3,
  "findings": [
    {
      "id": "source.required.present",
      "severity": "ok",
      "message": "required source \"client-acme\" is active",
      "source": "client-acme"
    }
  ]
}
```

| Field | Type | Notes |
|---|---|---|
| `project_root` | string | Absolute path to the audited git project. |
| `policy_path` | string | Absolute path to `.csaw/policy.yml` or `.csaw/policy.yaml`. Omitted when no policy is found. |
| `policy_found` | boolean | Whether a project policy was loaded. |
| `mounted` | number | Count of mounted entries inspected from csaw state or discovered links. |
| `findings` | array | Ordered findings produced by mount health and policy checks. |

## Finding Shape

```json
{
  "id": "source.required.ref_mismatch",
  "severity": "error",
  "message": "required source \"client-acme\" pin does not match policy",
  "source": "client-acme",
  "detail": "expected main, actual unpinned"
}
```

| Field | Type | Notes |
|---|---|---|
| `id` | string | Stable finding identifier. |
| `severity` | string | One of `ok`, `warn`, or `error`. |
| `message` | string | Human-readable summary. |
| `source` | string | Source name when the finding is source-specific. |
| `kind` | string | Artifact kind when the finding is kind-specific. |
| `path` | string | Project-relative path when the finding is path-specific. |
| `detail` | string | Extra diagnostic context. |

Optional fields are omitted when empty.

## Finding IDs

| ID | Severity | Meaning |
|---|---|---|
| `policy.loaded` | `ok` | A project policy was loaded. |
| `policy.missing` | `warn` | No `.csaw/policy.yml` or `.csaw/policy.yaml` was found. |
| `mount.none` | `warn` | No active csaw context was found. |
| `mount.healthy` | `ok` | All inspected mounted files are healthy. |
| `mount.unhealthy` | `error` | A mounted file is missing, replaced, drifted, points to a missing source, or has protected content drift. |
| `source.required.present` | `ok` | A required source is active and healthy. |
| `source.required.missing` | `error` | A required source is not active and healthy. |
| `source.required.metadata_missing` | `error` | A required source is active, but its configured metadata could not be found for URL verification. |
| `source.required.url_match` | `ok` | A required source URL matches policy. |
| `source.required.url_mismatch` | `error` | A required source URL does not match policy. |
| `source.required.ref_match` | `ok` | A required source project pin matches policy. |
| `source.required.ref_mismatch` | `error` | A required source project pin does not match policy. |
| `source.blocked.clear` | `ok` | No blocked sources are active. |
| `source.blocked.active` | `error` | A blocked source is active. |
| `kind.blocked.clear` | `ok` | No blocked artifact kinds are active. |
| `kind.blocked.active` | `error` | A blocked artifact kind is active. |
| `path.blocked.clear` | `ok` | No blocked mounted paths are active. |
| `path.blocked.active` | `error` | A blocked mounted path is active. |
| `kind.required.present` | `ok` | A required artifact kind is active and healthy. |
| `kind.required.missing` | `error` | A required artifact kind is not active and healthy. |

## Policy Fields

Create a starter policy with:

```bash
csaw audit --init
```

Overwrite an existing policy with:

```bash
csaw audit --init --force
```

`required_sources` supports source names or objects:

```yaml
required_sources:
  - team
  - name: client-acme
    url: git@example.com:org/client-acme-ai.git
    ref: main
```

`url` checks the configured source URL in csaw's source config. `ref` checks the project pin set by `csaw pin client-acme@main`; audit does not infer refs from the current branch of the source checkout.

`blocked_sources` supports exact names and glob patterns:

```yaml
blocked_sources:
  - personal-experimental
  - other-client-*
```

`blocked_kinds` uses the same kind names as `required_kinds`:

```yaml
blocked_kinds:
  - mcp
  - agents
```

`blocked_paths` supports exact project-relative paths, directory prefixes, and
glob patterns:

```yaml
blocked_paths:
  - .mcp.json
  - .claude/agents/**
```

`required_kinds` supports:

```yaml
required_kinds:
  - instructions
  - rules
  - agents
  - skills
  - mcp
```

## Mount Health Details

`mount.unhealthy` uses the `detail` field to identify the mount health issue:

| Detail | Meaning |
|---|---|
| `missing-source` | The recorded source file no longer exists. |
| `missing-link` | The project path recorded in mount state no longer exists. |
| `replaced-link` | The project path exists but is no longer the expected csaw-managed link. |
| `drifted-link` | The project path is a link, but it points somewhere other than the recorded source. |
| `protected-content-drift` | A protected mounted file no longer matches the SHA-256 hash recorded at mount time. |
| `protected-hash-unreadable` | csaw could not read a protected mounted file to verify its hash. |

Remount the context to accept an intentional protected source update and record a new protected hash.

## Example Reports

Passing report:

```json
{
  "project_root": "/path/to/project",
  "policy_path": "/path/to/project/.csaw/policy.yml",
  "policy_found": true,
  "mounted": 2,
  "findings": [
    {
      "id": "policy.loaded",
      "severity": "ok",
      "message": "project policy loaded",
      "path": "/path/to/project/.csaw/policy.yml"
    },
    {
      "id": "mount.healthy",
      "severity": "ok",
      "message": "2 mounted file(s) healthy"
    }
  ]
}
```

Failing report:

```json
{
  "project_root": "/path/to/project",
  "policy_path": "/path/to/project/.csaw/policy.yml",
  "policy_found": true,
  "mounted": 1,
  "findings": [
    {
      "id": "source.required.missing",
      "severity": "error",
      "message": "required source \"client-acme\" is not active",
      "source": "client-acme"
    },
    {
      "id": "source.blocked.active",
      "severity": "error",
      "message": "blocked source \"other-client-beta\" is active",
      "source": "other-client-beta",
      "detail": "matched other-client-*"
    }
  ]
}
```
