# Configure reviewers and the sink

Gnym loads one JSON configuration file for each review command. The file defines the reviewers to run and the sink that saves their collected results. The diff source remains a command-line argument because it changes from one review run to another.

## Configuration shape

A complete v0.1 configuration can include multiple reviewers, Markdown prompt files, and the file sink:

```json
{
  "reviewers": [
    {
      "name": "correctness",
      "type": "stub",
      "prompt_file": "prompts/correctness.md",
      "options": {
        "model": "development-model"
      }
    },
    {
      "name": "security",
      "type": "stub",
      "prompt_file": "prompts/security.md"
    }
  ],
  "sink": {
    "type": "file",
    "options": {
      "path": "review-results.json"
    }
  }
}
```

The stub reviewer accepts `model` as an optional string, but it does not use the model or prompt to analyze the diff. These fields exercise the configuration and routing boundary for future reviewer providers.

## Top-level fields

| Field | Required | Behavior |
| --- | --- | --- |
| `reviewers` | Yes | A non-empty array of reviewer configurations. Reviewers run sequentially in array order. |
| `sink` | Yes | The sink configuration used once after all reviewers succeed. |

The JSON file must contain exactly one value. Unknown fields are rejected rather than ignored.

## Reviewer fields

| Field | Required | Behavior |
| --- | --- | --- |
| `name` | Yes | Identifies the reviewer in routing and output. Names must be non-blank and unique. |
| `type` | Yes | Selects a registered reviewer provider. Version 0.1 supports only `stub`. |
| `prompt_file` | No | Loads the review prompt from a Markdown file. |
| `options` | No | Contains fields understood by the selected reviewer provider. |

For the `stub` reviewer, `options` supports only:

| Field | Required | Behavior |
| --- | --- | --- |
| `model` | No | An optional string copied into the runtime reviewer configuration. The stub does not call that model. |

Unknown reviewer options cause configuration of that reviewer to fail.

### Store a reviewer prompt in Markdown

Use `prompt_file` to keep a substantial prompt independent from the JSON configuration:

```markdown
# Correctness review

Review the supplied diff for correctness defects and unsafe error handling.
```

Relative prompt paths are resolved from the directory containing the JSON configuration file, not from the shell's current directory. Absolute prompt paths are also accepted. A configured prompt file must exist, be readable, and contain at least one non-whitespace character.

The current stub reviewer accepts a configuration without `prompt_file`. Future reviewer providers can apply their own requirements when they are registered.

## Sink fields

| Field | Required | Behavior |
| --- | --- | --- |
| `type` | Yes | Selects a registered sink provider. Version 0.1 supports only `file`. |
| `options` | Provider-specific | Contains fields understood by the selected sink provider. |

The `file` sink requires:

| Field | Required | Behavior |
| --- | --- | --- |
| `path` | Yes | File path where the JSON review run is written. It must not be blank. |

A relative sink path is resolved from the Gnym process working directory. The Compose `go` service uses `/workspace`, the repository root. Saving replaces the contents of an existing file at that path. Parent directories are not created automatically.

The output is an indented version-1 JSON envelope with `schema_version`, `status`, `reviews`, and `failures`. Successful runs contain one accepted result per configured reviewer in configuration order. See the [review output contract](review-schema.md) for fields, validation, and migration from the former result array. The sink validates the aggregate before writing; writes are not atomic.

## Diff source argument

Pass the diff source separately when running a review:

```sh
docker compose run --rm go go run . review \
  --config path/to/gnym.json \
  --diff-source file://changes.diff
```

Version 0.1 registers only the `file` diff source. These forms are supported:

| Reference | Resolved file path |
| --- | --- |
| `file:changes.diff` | `changes.diff` |
| `file://changes.diff` | `changes.diff` |
| `file:///tmp/changes.diff` | `/tmp/changes.diff` |

Use `file://name.diff` for a path relative to the process working directory and `file:///absolute/path.diff` for an absolute path. A reference without a scheme is rejected. A reference containing both a host and a path, such as `file://host/path`, is also rejected by the v0.1 parser.

## Validation and failure behavior

Configuration loading fails before the diff is read when:

- the configuration file cannot be opened or decoded;
- the JSON includes an unknown field or more than one value;
- no reviewers are configured;
- a reviewer has a blank name or type;
- reviewer names are duplicated;
- the sink type is blank; or
- a prompt file is missing, unreadable, or empty.

Pipeline construction then validates registered types and provider-specific options. Unsupported providers produce an error such as `reviewer type "anthropic" is not registered` or `sink type "github" is not registered`.

During execution, Gnym stops on the first diff-source, reviewer, or result-validation error and does not call the sink. Earlier successful reviews are not saved on that failure path. A sink error is returned after the completed review run is passed to the sink. The current CLI prints errors to standard error with a `gnym:` prefix and exits with status 1. See [validation failures](review-schema.md#validation-failures) for result diagnostics and recovery.

Gnym v0.1 does not load a `.env` file or define credentials in its JSON contract because none of the registered providers require secrets.
