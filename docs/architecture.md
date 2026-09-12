# Understand Gnym's architecture

This page describes the implemented v0.1 architecture for contributors extending the review pipeline. Gnym currently provides local file and stub implementations; external AI and hosted-code providers are not implemented.

## Review pipeline

A review run follows one control-flow path:

```text
CLI arguments + JSON configuration
                 |
                 v
         build providers
                 |
                 v
file diff source -> coordinator -> reviewer router -> configured reviewers
                       |
                       v
                   file sink
                       |
                       v
          version-1 JSON review run
```

The CLI loads and validates configuration, selects factories from the registered provider types, constructs the runtime dependencies, and passes them to the coordinator. The coordinator contains no JSON, filesystem-path, or command-line parsing logic.

## Package responsibilities

### `config`

The `config` package defines the shared JSON shape, rejects unknown fields, validates fields common to every provider, and loads Markdown prompts. It preserves each provider's `options` as raw JSON.

Provider-specific validation does not belong in this package. The factory selected for a reviewer or sink decodes and validates its own options when the CLI constructs that provider.

### `input`

The `input` package contains diff-source implementations. `FileDiffSource` reads the configured file and returns its entire contents as a `review.Diff`.

### `reviewer`

The `reviewer` package defines reviewer configuration, requests, provider-owned payloads, accepted results, comments, and severities. It extracts represented filenames from bounded diff metadata and validates payloads before accepting them. Acceptance supplies the configured reviewer identity and a UTC timestamp. Its current `Stub` implementation returns `Stub review completed` with zero comments. It does not analyze the request's diff or prompt.

### `review`

The `review` package owns runtime contracts and orchestration:

```go
type DiffSource interface {
    GetDiff() (Diff, error)
}

type Reviewer interface {
    Review(request reviewer.Request) (reviewer.Payload, error)
}

type CommentSink interface {
    Save(run Run) error
}
```

`Coordinator.Run` validates that at least one reviewer is present, fetches the diff once, extracts represented filenames, and executes configured reviewers sequentially. Each payload passes structure and file-membership validation before Gnym supplies trusted identity and acceptance time. The coordinator validates and saves one `complete` review run after all reviewers succeed. It stops immediately on a diff-source, reviewer, or validation error without calling the sink, even if earlier reviewers succeeded.

The [shared output contract](review-schema.md) separates provider content from Gnym metadata. Location checks preserve structurally valid inaccurate line references; the filename set does not map findings to hunks. Checked-in schemas describe wire structure, while Go validation enforces additional semantics. Provider adapters must return the shared payload, not choose result identity or timestamps.

### `sink`

The `sink` package contains output implementations. The file sink validates aggregate structure before writing the entire review run as indented JSON. It does not receive the diff or repeat membership validation. Writes replace existing file contents and are not atomic. Sink errors propagate; the run's status describes reviewer outcomes, not successful delivery.

### `main`

The main package is the application boundary. It parses `review` and `version`, owns the provider factory registries, converts configuration into runtime dependencies, and maps a returned error to CLI output and exit status 1.

## Configuration and runtime boundaries

Configuration values and runtime dependencies remain separate:

1. `config.Load` decodes the shared structure and loads prompt text.
2. The CLI looks up the configured `type` in the appropriate factory registry.
3. The selected factory validates its provider-specific `options` and builds an implementation of a runtime contract.
4. The coordinator receives constructed interfaces and runtime reviewer configurations.

This boundary keeps the coordinator independent of configuration files and lets different providers define different option shapes without coupling them to one global schema.

## Reviewer routing

The coordinator accepts one `review.Reviewer` boundary plus an ordered list of reviewer configurations. The CLI supplies a reviewer router behind that boundary. Each configured reviewer is constructed independently and registered in the router by its unique reviewer name.

For every request, the router uses `request.Config.Name` to select the constructed reviewer. This allows differently configured reviewer providers to share the current coordinator contract without moving provider selection into the orchestration package.

## Provider extension points

The main package currently has separate factory registries for diff sources, reviewers, and sinks:

- `file` diff source;
- `stub` reviewer; and
- `file` sink.

Adding a provider requires an implementation of the relevant runtime interface, a factory that validates its options, and registration under a type name. The registry is currently compiled into the main package; configuration cannot dynamically load an unregistered provider.

The factory boundary can support provider-specific requirements in the future, but v0.1 does not implement OpenAI, Anthropic, GitHub, Bitbucket, or any other external integration. It also does not load environment files or credentials.

## Error and lifecycle behavior

Gnym performs work in a fixed sequence:

1. Parse and validate CLI arguments.
2. Load and validate the configuration and prompt files.
3. Construct the diff source, reviewers, and sink.
4. Fetch the diff once.
5. Run reviewers sequentially in configuration order.
6. Save the complete result once.

There is no retry, timeout, cancellation, parallel review, or partial-result persistence in v0.1. Errors propagate to the CLI, which prints them with a `gnym:` prefix and exits with status 1. Successful commands return normally with status 0.

## Verify changes

Run ordinary Go verification through the `go` service:

```sh
docker compose run --rm go go test -count=1 ./...
```

Run mutation testing through the specialized `go-testing` service:

```sh
docker compose run --rm go-testing gremlins unleash
```

Mutation testing is diagnostic evidence, not a requirement to mutate every process-boundary line. The small `main` wrapper delegates meaningful command behavior to testable functions; uncovered mutations in that wrapper do not by themselves imply a missing product behavior test.
