# Get your first review result

This guide runs Gnym's local v0.1 pipeline and produces a JSON review result. The current stub reviewer returns deterministic sample feedback; it does not analyze the diff with an AI model.

## Prerequisites

You need:

- a checkout of the Gnym repository;
- Docker with Compose available; and
- a terminal open in the repository root.

Gnym's Go commands run through the checked-in Compose services, so you do not need a host Go installation.

## 1. Create a diff

Save a Git diff as `changes.diff` in the repository root. For example, capture the current working-tree changes:

```sh
git diff > changes.diff
```

Gnym reads the file contents as the diff for the review run. An empty file is accepted by the current file diff source, although it does not represent a useful review input.

## 2. Configure the pipeline

Create `gnym.json` in the repository root:

```json
{
  "reviewers": [
    {
      "name": "correctness",
      "type": "stub"
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

This configuration names one stub reviewer and tells the file sink to write `review-results.json`. The sink path is relative to Gnym's working directory, which is the repository root in the `go` service.

## 3. Run the review

Run Gnym through the project container:

```sh
docker compose run --rm go go run . review \
  --config gnym.json \
  --diff-source file://changes.diff
```

The command returns without terminal output when the review and save both succeed.

If Gnym reports `open changes.diff: no such file or directory`, confirm that `changes.diff` exists in the repository root. Both relative paths in this example are resolved from the container's `/workspace` working directory.

## 4. Verify the result

Open `review-results.json`. The current stub reviewer produces a complete version 1
run. The exact `created_at` value is the UTC time when Gnym accepted the result:

```json
{
  "schema_version": "1",
  "status": "complete",
  "reviews": [
    {
      "reviewer": "correctness",
      "created_at": "2026-10-19T14:55:17.123456789Z",
      "summary": "Stub review completed",
      "comments": []
    }
  ],
  "failures": []
}
```

The empty comments array is intentional: the stub verifies routing and output without
inventing a finding unrelated to the supplied diff. The result confirms that Gnym
read the diff, routed the configured reviewer, accepted its payload, and saved the
review run.

## Check the version

To print the current CLI version, run:

```sh
docker compose run --rm go go run . version
```

The v0.1 command prints:

```text
gnym v0.1.0
```

Next, see [Configure reviewers and the sink](configuration.md) to add prompt files,
multiple reviewer perspectives, and provider-specific options supported by v0.1.
See [Use the version 1 review schema](review-schema.md) for the provider payload,
output contract, migration notes, and validation boundaries.
