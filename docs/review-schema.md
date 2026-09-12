# Read the review output contract

Use this reference to consume Gnym's JSON output or implement a reviewer or sink. The current pipeline emits a version-1 `complete` review run after every configured reviewer succeeds. Start with [your first review result](getting-started.md) for an executable stub example.

## Review run envelope

All four fields are required. Unknown fields, incompatible types, JSON `null`, and unknown schema versions are rejected.

| Field | Contract |
| --- | --- |
| `schema_version` | The string `"1"`, not a number. |
| `status` | `complete`, `partial`, or `failed`; describes reviewer outcomes, not sink delivery. |
| `reviews` | Array of accepted reviewer results. |
| `failures` | Array of failure records. |

| Status | Reviews | Failures | Current command behavior |
| --- | --- | --- | --- |
| `complete` | One or more | Empty | Emitted after all reviewers succeed. |
| `partial` | One or more | One or more | Representation supported; not emitted by the coordinator. |
| `failed` | Empty | One or more | Representation supported; not emitted by the coordinator. |

Both arrays empty is invalid. Reviewer identities must be unique across both arrays; one reviewer cannot appear in both. Arrays preserve their supplied order. Runtime reviews follow configuration order. Required collections serialize as arrays, including empty arrays, never `null`.

Each failure has exactly three required nonblank strings: `reviewer`, `code`, and `message`. Version 1 permits `invalid_output` for shared-contract failures and `reviewer_error` for other reviewer failures.

The following is an illustrative constructed run, not current CLI failure output:

```json
{
  "schema_version": "1",
  "status": "failed",
  "reviews": [],
  "failures": [
    {
      "reviewer": "correctness",
      "code": "invalid_output",
      "message": "comments[0].file: not represented in diff metadata"
    }
  ]
}
```

Continued execution after reviewer failures and runtime `partial`/`failed` delivery are outside the current feature. Today, the first reviewer or validation failure stops execution without calling the sink, including when earlier reviewers succeeded.

## Provider payload and accepted result

A provider supplies exactly `summary` and `comments`. The summary must be a nonblank string and comments must be an array. Zero findings is `[]`. Each comment requires `file`, `severity`, and a nonblank `message`; location fields are optional as described below. Accepted summary and message text is preserved without trimming or rewriting. One invalid comment rejects the entire payload; invalid comments are not silently dropped.

This illustrative provider payload contains a file-wide finding and a line-range finding. It requires `service.go` to be represented in the supplied diff metadata; the current stub does not generate these findings:

```json
{
  "summary": "The service needs stronger error handling.",
  "comments": [
    {
      "file": "service.go",
      "severity": "medium",
      "message": "Document how callers distinguish temporary failures."
    },
    {
      "file": "service.go",
      "side": "new",
      "line": 42,
      "end_line": 44,
      "severity": "high",
      "message": "Return the write error before reporting success."
    }
  ]
}
```

Gnym adds two required fields to an accepted result:

| Field | Source and meaning |
| --- | --- |
| `reviewer` | The configured reviewer's name, supplied by Gnym. |
| `created_at` | UTC acceptance time in RFC3339 form ending in `Z`; fractional seconds are allowed. |

Provider payloads cannot set these fields. The timestamp records acceptance, not provider execution start or successful output delivery.

### Severity

| Value | Meaning |
| --- | --- |
| `low` | Minor improvement with little functional impact. |
| `medium` | Meaningful weakness with limited consequences. |
| `high` | Substantial functional or security impact. |
| `critical` | Critical impact requiring urgent attention, such as exposed usable credentials or sensitive-data disclosure. |

Severity expresses supported impact, not confidence. Prompts determine whether cleanup findings are relevant. Values are exact and lowercase; legacy `info`, `warning`, and `error` values are invalid.

## Locations and represented files

For a file-wide finding, omit `side`, `line`, and `end_line`. For a line finding, supply `side` (`old` or `new`) and positive `line` together. Optional `end_line` is inclusive and must be at least `line`. A lone side, lone line, or end line without the required pair is invalid.

Line numbers must be integral values from 1 through 9223372036854775807. JSON decoding accepts exact integral decimal or exponent forms such as `1.0` and `1e0`; output uses integers. Fractional values and overflow are rejected without rounding. Consumers must preserve signed 64-bit precision.

Line placement is advisory. Gnym does not check whether the line exists in a hunk, whether the range crosses hunks, or whether the side is semantically correct. Structurally valid locations remain unchanged: Gnym does not adjust, drop, or downgrade them. Exact placement for inline publishing is outside the current feature.

The file must be a repository-relative slash-separated path and match an extracted filename exactly, including case. Absolute or drive-qualified paths, empty components, `.` or `..` components, backslashes, control characters, and invalid UTF-8 are rejected. Validation does not read repository files.

### Recognized diff metadata

Gnym extracts a filename set from these forms:

- Conventional `diff --git` headers with `a/` and `b/` paths.
- Paired `---` and `+++` file headers outside hunk bodies, including standalone unified patches. Conventional `a/` and `b/` prefixes are stripped; tab-delimited timestamps are excluded.
- Rename and copy source/destination metadata within Git file sections.
- Added and deleted files; `/dev/null` is never a commentable filename.
- Binary and mode-only Git sections with recognizable header names.
- Git quoted paths and ordinary filenames containing spaces when metadata is unambiguous.

Both old and new names are included when recognized; membership is independent of a finding's side. Hunk body text is not treated as file metadata. Ambiguous or unsupported metadata supplies no guessed names. Combined diffs, custom prefixes, and unusual non-Git patch dialects are outside the supported boundary.

Extraction does not reject the diff itself. Empty files and arbitrary text remain accepted input, and zero findings remains valid for either. A finding whose file cannot be established fails membership validation. This checks references against supplied metadata; it does not prove the input represents an actual repository change. Input remains memory-bound; there is no new size cap or full diff validation.

## JSON and adapter boundaries

Public JSON uses lowercase snake_case keys. Missing required fields, unknown fields, incompatible types, and `null` are rejected. Absent location fields are omitted, not represented by `null` or zero. In-memory Go nil collections normalize to empty arrays when serialized; this does not make JSON `null` valid input.

A future provider adapter whose API requires nullable location fields may map those nulls to absence before shared validation. This is an adapter responsibility, not automatic shared decoding behavior. Other invalid values must not be repaired. No external AI adapter is implemented by this change.

The checked-in [provider payload schema](../schemas/reviewer-payload.schema.json) and [review run schema](../schemas/review-run.schema.json) use JSON Schema draft 2020-12. They cover structure, enums, required fields, location dependencies, numeric limits, and aggregate cardinalities. Shared Go validation additionally enforces nonblank text, lexical paths, file membership during acceptance, range ordering, unique identities, and valid UTC timestamps. A JSON Schema check alone does not establish acceptance. Go aggregate validation also enforces status/cardinality consistency.

## Validation failures

Diagnostics identify the reviewer and offending field or rule, with a comment index where applicable. For example, `comments[0].file: not represented in diff metadata` means Gnym could not establish the first finding's file from supported metadata. Check exact filename spelling and case, and ensure the supplied patch includes recognizable file headers. Regenerate unsupported patch formats as a conventional Git diff where appropriate.

A location-structure error instead calls for a complete `side`/`line` pair and a valid positive ordered range. Moving an otherwise valid line into a hunk is not required. Severity or required-field errors must be corrected by the reviewer implementation. Shared validation diagnostics avoid echoing complete diffs, prompts, or payloads.

The current CLI reports the error and exits with status 1. It saves no run on reviewer or validation failure, so an existing output file may still contain an earlier run. Check command success before consuming that file. A file sink validates aggregate structure before writing, without repeating membership checks because it has no diff. I/O failures propagate; missing parent directories are not created and writes are not atomic.

## Migrate from the former output

The envelope replaces the previous PascalCase result array directly; there is no legacy mode.

1. Read results from `reviews` instead of the top-level array, and check `schema_version` as a string.
2. Use lowercase fields such as `reviewer`, `summary`, `comments`, `file`, and `message`. Read the new `created_at` acceptance timestamp.
3. Update severity producers and consumers to `low`, `medium`, `high`, and `critical`. There is no automatic mapping from legacy severities.
4. Support file-wide findings with omitted location fields. Line findings now require `side` with `line` and may include `end_line`.
5. Expect the stub summary `Stub review completed` and `comments: []`; the former fabricated `stub.go` finding and misspelled summary are gone.

For extension responsibilities and execution order, see [Gnym's architecture](architecture.md).
