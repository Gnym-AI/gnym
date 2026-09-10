# Gnym

Gnym is an early code-review pipeline that reads a diff, runs configured reviewers, and writes the collected review results to a sink.

Version 0.1 provides an executable pipeline for local development. It supports a file diff source, a deterministic stub reviewer, and a JSON file sink. It does not yet call an AI provider, fetch a hosted pull request, or publish review comments.

## Run the current pipeline

With `gnym.json` and `changes.diff` in the repository root, run:

```sh
docker compose run --rm go go run . review \
  --config gnym.json \
  --diff-source file://changes.diff
```

The file sink writes the configured JSON output after all configured reviewers complete.

## Documentation

- [Get your first review result](docs/getting-started.md)
- [Configure reviewers and the sink](docs/configuration.md)
- [Understand Gnym's architecture](docs/architecture.md)
