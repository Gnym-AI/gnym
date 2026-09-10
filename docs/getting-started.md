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

Open `review-results.json`. The current stub reviewer produces this JSON shape:

```json
[
  {
    "Reviewer": "correctness",
    "Summary": "Stub reivew completed",
    "Comments": [
      {
        "File": "stub.go",
        "Line": 1,
        "Severity": "warning",
        "Message": "This is a stub review"
      }
    ]
  }
]
```

The misspelling in the stub summary is part of the current v0.1 output. The result confirms that Gnym read the diff, routed the configured reviewer, collected its result, and saved the review run.

## Check the version

To print the current CLI version, run:

```sh
docker compose run --rm go go run . version
```

The v0.1 command prints:

```text
gnym v0.1.0
```

Next, see [Configure reviewers and the sink](configuration.md) to add prompt files, multiple reviewer perspectives, and provider-specific options supported by v0.1.

