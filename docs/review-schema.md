# Use the version 1 review schema

Gnym uses one provider-independent payload and one aggregate output envelope. The
checked-in JSON Schemas are
[`reviewer-payload.schema.json`](../schemas/reviewer-payload.schema.json) and
[`review-run.schema.json`](../schemas/review-run.schema.json). Both use JSON Schema
Draft 2020-12 and resolve without a network request.

## Provider payload

A reviewer provider returns only its summary and findings. It cannot choose the
reviewer identity or acceptance timestamp:

```json
{
  "summary": "One correctness issue found",
  "comments": [
    {
      "file": "internal/cache/store.go",
      "side": "new",
      "line": 42,
      "end_line": 44,
      "severity": "high",
      "message": "The error path leaves the cache entry locked."
    },
    {
      "file": "README.md",
      "severity": "low",
      "message": "The documented default no longer matches the configuration."
    }
  ]
}
```

`summary` and `comments` are required. A successful review with no findings uses
`"comments": []`, never `null`. Summaries and messages must contain a non-whitespace
character. Accepted text is preserved rather than trimmed or rewritten. Unknown
fields, missing fields, incompatible types, and one invalid comment reject the
entire payload.

The severity values describe impact, not confidence:

| Severity | Intended impact |
| --- | --- |
| `low` | Minor improvement with little functional impact. |
| `medium` | Meaningful weakness with limited consequences. |
| `high` | Substantial functional or security impact. |
| `critical` | Urgent critical impact, such as usable credential or sensitive-data exposure. |

### Locations

A comment is either file-wide or line-specific:

- A file-wide comment omits `side`, `line`, and `end_line`.
- A line comment has `side` equal to `old` or `new` and a positive signed 64-bit
  `line`. An optional inclusive `end_line` must be at least `line`.
- Absent location fields are omitted. They are not emitted as `null`.

Locations are advisory. Gnym validates their structure but does not check that a
line exists in a hunk, that a range stays inside a hunk, or that the selected side
is semantically correct. Structurally valid out-of-hunk and cross-hunk locations
remain unchanged. Exact inline-placement validation belongs to a future publishing
boundary.

### File membership

Every commented file must be established by supported metadata in the supplied
diff. Matching is exact and case-sensitive. Gnym recognizes:

- conventional `diff --git a/... b/...` headers;
- paired `---` and `+++` unified headers before a hunk;
- Git rename and copy source/destination metadata;
- added and deleted files, excluding `/dev/null`;
- recognizable binary and mode-only Git sections; and
- ordinary spaces and Git-quoted path escapes.

Old and new names are both eligible. Paths must be repository-relative,
slash-separated UTF-8 without empty, `.` or `..` components, backslashes, drive or
absolute prefixes, or control characters. Validation is lexical and never reads the
repository filesystem.

Metadata extraction is deliberately bounded. It does not infer names from hunk
bodies, combined diffs, custom prefixes, or unusual patch dialects. Unsupported or
arbitrary input is not itself an error, and zero findings can still succeed. A
finding for a file that the recognized metadata cannot establish fails distinctly.

## Accepted results and complete runs

After validating a payload and file membership, Gnym supplies the configured
`reviewer` name and `created_at`. The timestamp is the UTC acceptance time in
RFC 3339 form with a trailing `Z`; fractional seconds are allowed. Providers cannot
override either field.

The coordinator preserves configuration order and writes one complete envelope:

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

All four top-level fields are required. `reviews`, `failures`, and nested `comments`
are always arrays, never `null`. Version 1 accepts these status/cardinality pairs:

| Status | Reviews | Failures |
| --- | --- | --- |
| `complete` | One or more | None |
| `partial` | One or more | One or more |
| `failed` | None | One or more |

Reviewer identities must be unique across both arrays. Each failure has a nonblank
`reviewer`, `message`, and one of two codes: `invalid_output` for a shared-contract
failure or `reviewer_error` for another reviewer failure.

Partial and failed envelopes are valid representable shapes, for example:

```json
{
  "schema_version": "1",
  "status": "failed",
  "reviews": [],
  "failures": [
    {
      "reviewer": "security",
      "code": "reviewer_error",
      "message": "provider request failed"
    }
  ]
}
```

The current runtime does not produce partial or failed files. Continued execution,
error-to-failure mapping, and persistence of these outcomes belong to GNY-8. Today,
the coordinator remains sequential and fail-fast: a diff-source, reviewer, or
payload-acceptance error prevents any sink call.

## Validation and sink boundaries

JSON Schema covers required fields, types, enums, unknown-field rejection, location
dependencies, numeric bounds, and aggregate cardinalities. Go validation additionally
enforces nonblank text, safe repository-relative paths, recognized diff membership,
range order, UTC timestamps, unique reviewer identities, and advisory-location
semantics.

Validation errors identify the reviewer and failing field or rule, including the
comment index when applicable. They do not echo the full diff, prompt, or provider
response. The CLI prints the returned error with a `gnym:` prefix and exits with
status 1.

The file sink validates aggregate structure again before writing. It does not have
the diff and therefore does not repeat membership validation. A valid run replaces
the configured file with indented JSON. Writes are not promised to be atomic, parent
directories are not created, and simultaneous writers retain existing filesystem
behavior.

## Migrate earlier output

Version 1 directly replaces the earlier unversioned PascalCase result array; there
is no compatibility mode because that format had no users. Consumers must:

- read the top-level envelope instead of a bare array;
- use `reviews` and `failures`, plus `schema_version` and `status`;
- replace `Reviewer`, `Summary`, and `Comments` with lowercase snake_case fields;
- accept trusted `created_at` on every review;
- replace legacy `info`, `warning`, and `error` severities with `low`, `medium`,
  `high`, or `critical`; and
- treat omitted location fields as file-wide instead of expecting placeholder or
  `null` values.

Provider-specific decoding and API schemas remain provider-adapter concerns. Version
1 does not add retries, timeouts, cancellation, concurrency, atomic writes, exact
hunk validation, or runtime partial/failed delivery.
