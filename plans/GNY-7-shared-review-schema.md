# GNY-7: Shared reviewer and run schema

## 1. Status and revision

Revision 2 — delivery decomposition approved by the developer on 2026-09-17.

Revision 2 changes only the executable Task breakdown. The approved behavior,
acceptance criteria, verification requirements, scope, non-goals, failure semantics,
compatibility, and security boundaries remain unchanged from revision 1.

Story: GNY-7. Parent Epic: GNY-1. Downstream Stories: GNY-8 orchestration,
GNY-3 OpenAI adaptation, GNY-4 sinks.

Developer decisions:

- Replace existing output directly; no legacy mode is needed because there are no current users.
- Preserve findings with inaccurate line placement. Validate location structure and file membership, but defer matching lines/ranges to diff hunks until inline publishing.
- Proceed with this contract through implementation, independent verification, and documentation.

## 2. Feature summary

Give Gnym one provider-independent reviewer payload and one versioned aggregate
representation. Validate and normalize successful results before delivery. Preserve
useful findings whose line numbers do not match the supplied hunks.

## 3. Problem and intended outcome

Current output has no version, acceptance timestamp, file-wide representation, or
common validation boundary. Its stub emits an unrelated location. Future providers
and sinks need a shared contract without letting prompts dictate the output format.

A successful command writes a validated version-1 run envelope. Invalid reviewer
payloads produce identifiable validation failures rather than successful empty reviews.

## 4. Current behavior and repository evidence

Inspected at `bce3aa8a14a77bc2207ea61fb7b6eeb69ef3905e`:

- `reviewer/types.go`: results contain reviewer, summary, comments; comments contain file, integer line, severity, message. Severities are info/warning/error.
- `reviewer/stub.go`: fixed `stub.go` finding unrelated to input.
- `review/coordinator.go`: reads the diff once, executes sequentially, stops at first failure, saves only after all reviewers succeed.
- `review/types.go`: run contains only results.
- `sink/file.go`: writes a PascalCase JSON result array with `os.WriteFile`.
- `input/file.go`: returns arbitrary file contents, including empty text; no diff parser exists.
- Configuration rejects zero reviewers and duplicate reviewer names.
- Existing tests protect sequential execution and fail-fast behavior.

The orchestrator verified the existing full container test suite passes.

## 5. Desired behavior

### Reviewer payload and accepted result

The logical provider payload contains required `summary` and `comments`.

- `summary`: nonblank string.
- `comments`: array; zero findings is `[]`.
- Each comment requires `file`, `severity`, and nonblank `message`.
- Severity is exactly `low`, `medium`, `high`, or `critical`.
- low: minor improvement with little functional impact.
- medium: meaningful weakness with limited consequences.
- high: substantial functional or security impact.
- critical: critical impact requiring urgent attention, such as exposed usable credentials or sensitive-data disclosure.
- Severity describes supported impact, not confidence. Prompts determine whether cleanup findings are relevant.
- Preserve summary/message text; do not trim or rewrite accepted content.
- Reject missing required fields, incompatible types, unsupported severity, and unknown payload/comment fields.
- Do not silently discard invalid comments. One invalid comment rejects the result.

An accepted result additionally contains Gnym-supplied `reviewer` (configured name)
and `created_at` (UTC acceptance time, RFC3339 with `Z`; fractional seconds allowed).
Provider payloads exclude these fields and cannot choose identity or acceptance time.
Internal interfaces may change to enforce this boundary.

Published JSON uses lowercase snake_case keys. Required arrays are always arrays,
never `null`. A nil in-memory comments collection may normalize to `[]`.

### Location structure

`file` is a nonempty repository-relative slash-separated path, matched exactly and
case-sensitively after extraction. Reject absolute/drive-qualified paths, empty
components, `.` or `..` components, backslashes, control characters, and invalid
UTF-8. Do not access the filesystem to validate it.

A comment has either:

- File-wide location: omit `side`, `line`, and `end_line`.
- Line location: `side` is `new` or `old`; `line` is positive; optional inclusive `end_line` is greater than or equal to `line`.

A lone side, lone line, or end line without the required pair is invalid. Integers
must fit signed 64-bit integers.

Do not check whether lines exist in a hunk, whether ranges cross hunk boundaries,
or whether the chosen side is semantically correct. Do not adjust, drop, or
downgrade an otherwise valid location for these reasons.

Public JSON omits absent location fields, rather than emitting `null`. Future
provider adapters may map API-required null locations to absence before validation.
Other invalid values must not be repaired. OpenAI-specific schema/decoding belongs
to GNY-3.

### File membership without validating the entire diff

Require every comment file to occur in recognized file metadata. Build a name set,
not a hunk/line map. Support:

- Conventional `diff --git` headers using `a/` and `b/` paths.
- Paired `---` / `+++` headers before a hunk.
- Rename/copy source and destination metadata within a Git file section.
- Added/deleted files, excluding `/dev/null` as a commentable name.
- Binary and mode-only Git sections when recognizable header names exist.
- Git quoted path escaping and ordinary filenames containing spaces.

Include old and new names for renames/deletions; membership is independent of side.
Remove only conventional patch prefixes, not arbitrary path components. Support
standalone unified header pairs, stripping conventional `a/` and `b/` prefixes when
present. Tab-delimited timestamps are not part of filenames.

Do not interpret hunk body lines as metadata. Ambiguous/unsupported metadata yields
no guessed names. Combined diffs, custom prefixes, and unusual non-Git patch
dialects need not yield membership.

Extraction never rejects the input itself. Arbitrary text and empty input remain
accepted. Zero findings remains valid for either. A comment whose file cannot be
established fails file-membership validation, with a diagnostic distinct from line
structure. Document this bounded support. This checks references against supplied
input, not whether the input truthfully represents a repository.

### Aggregate contract

Required fields:

```json
{"schema_version":"1","status":"complete","reviews":[],"failures":[]}
```

The illustrative field shape above requires these cardinalities to be valid:

- `complete`: one or more reviews, zero failures.
- `partial`: one or more reviews and one or more failures.
- `failed`: zero reviews and one or more failures.
- Both arrays empty is invalid; zero reviewers remains a preflight error.

Each failure contains exactly nonblank `reviewer`, `code`, and `message`.
Version-1 codes are `invalid_output` (shared contract failure) and `reviewer_error`
(other reviewer failure). Finer codes require an explicit contract revision.
GNY-8 assigns runtime errors to records.

Reviewer identities must be unique across both arrays. A reviewer cannot appear
in both. Preserve input order within each array; runtime reviews follow configuration
order. Status describes reviewer outcomes, never sink delivery.

Provide checked-in JSON Schema for provider payload and aggregate representation,
covering structure, enums, omissions, and required fields. Document semantic checks
beyond JSON Schema: membership, nonblank text where needed, range order, unique
identities, UTC timestamps, and status/cardinality consistency. Tests prevent drift.

### Runtime and delivery boundary

GNY-7 keeps existing preflight/diff-fetch behavior; validates returned payloads;
supplies trusted identity/time; stops at the first reviewer or validation failure
without sink invocation; saves one `complete` envelope after all reviewers succeed.
The stub returns deterministic summary `Stub review completed` and zero comments.

The file sink serializes the entire validated envelope. Direct sink calls reject
invalid aggregate structure before writing. Membership has already been checked;
the sink does not receive the diff or repeat membership validation.

GNY-8 owns continuing after errors and runtime partial/failed output. GNY-7 tests
these shapes with constructed runs.

Validation diagnostics identify reviewer, field/rule, and comment index where
applicable, without echoing full diff, prompt, or raw model response. Existing CLI
prefix and exit 1 remain; success remains exit 0 with no new terminal output.

## 6. Scope, non-goals, and deferred work

Included: shared types/schema, validation, filename extraction, trusted metadata,
complete-run integration, file sink migration, stub correction, verification, docs.

Deferred: partial/failed runtime delivery and precedence (GNY-8); provider parsing,
Structured Outputs, credentials/network (GNY-3); exact hunk placement; retries,
timeouts, cancellation, concurrency, streaming, severity exits; atomic file
replacement and permission/retention redesign; general-purpose diff parsing and
repository-content inspection.

## 7. Decisions and constraints

Developer location decisions supersede earlier strict-line recommendations.
Validate at the shared acceptance boundary, not only in providers. Keep orchestration
free of filesystem and JSON parsing. No legacy option, new CLI/configuration fields,
external services, or host Go requirements. Schemas must not need network fetching
at runtime or during tests. Existing file writes may be partial on I/O failure;
this story does not promise atomic delivery.

## 8. Adaptive readiness review

| Category | Disposition |
| --- | --- |
| Functional | Specified: payload, locations, membership, envelope, order, stub, success integration. |
| Failure | Specified: whole-result rejection, distinct diagnostics, retained fail-fast behavior. |
| Lifecycle | Specified: synchronous sequential execution; validation before acceptance. New timeout/cancellation guarantees deferred. |
| Retries/idempotency | Not applicable to new behavior: no retries/requests added; reruns retain overwrite behavior. |
| Resources | Specified: existing in-memory diff/results; no validation network/process/file lookup; do not expand line ranges. No new caps; large input remains memory-bound. |
| Concurrency | Specified: no new concurrency or mutable global per-run validation state; simultaneous output writers retain existing behavior. |
| Environment | Specified: existing Go/Docker environment; no credentials or runtime network dependency. |
| Persistence | Specified: new envelope replaces old format; non-null arrays; existing permissions. Atomicity deferred. |
| Security | Specified: untrusted payload validation, trusted metadata, lexical paths, bounded diagnostics, no repository reads; metadata is not authenticity proof. |
| Observability | Specified: reviewer/rule/index diagnostics; existing CLI behavior. Metrics/tracing not applicable. |
| Compatibility | Specified: direct format/severity break approved; no legacy mode; unknown schema versions rejected. |
| Documentation | Specified: schema/migration/limitations/examples/architecture and introductory output. |

## 9. Acceptance criteria

- **AC-1:** Valid payloads produce lowercase results with trusted identity, UTC acceptance time, non-null comments.
- **AC-2:** Invalid structure rejects whole result with bounded diagnostics; unknown fields and legacy severities fail.
- **AC-3:** Supported metadata establishes membership; positive structurally valid out-of-hunk, cross-hunk, and misplaced locations remain unchanged.
- **AC-4:** Extraction accepts arbitrary/empty input; zero findings succeeds; unknown files fail distinctly.
- **AC-5:** Aggregate shapes enforce version, cardinalities, identities, arrays, codes, and structure.
- **AC-6:** Success yields one complete envelope in order; reviewer/validation failure remains fail-fast with no sink call.
- **AC-7:** File output adopts schema directly, rejects invalid runs before writing, propagates I/O errors.
- **AC-8:** Schemas/docs match verified behavior and distinguish deferred orchestration/placement work.

## 10. Required verification

- **VR-1 (AC-1/2):** Payload matrix: omissions, types, enums, blanks, unknown fields, location combinations, integer boundaries, reversed ranges, nil collections, identity spoofing, UTC time; invalid comments never silently dropped.
- **VR-2 (AC-3/4):** Membership fixtures: modify/add/delete/rename/copy, binary/mode-only, spaces/quoted names, standalone headers, body resembling metadata, unsafe paths, empty/arbitrary/unsupported input. Preserve out-of-hunk lines, cross-hunk ranges, wrong semantic sides.
- **VR-3 (AC-5):** Complete/partial/failed runs; empty/inconsistent/duplicate/unknown version/code/missing/null cases; schema examples and schema/runtime agreement with semantic-only constraints documented.
- **VR-4 (AC-1/6):** Independent coordinator evidence for ordering, trusted stamping, success, first/later invalid payload, reviewer/preflight/diff/sink errors; invocation counts and no delivery on review failure.
- **VR-5 (AC-7/8):** File/CLI integration independently decodes actual envelope instead of building expectations with production marshaling; no write on invalid aggregate; I/O failure.
- **VR-6 (all):** `docker compose run --rm go go test -count=1 ./...`; targeted mutation checks of meaningful validation/orchestration via `go-testing`, reporting relevant survivors and implications without blanket score.

No live provider verification required.

## 11. Smallest coherent implementation tasks

All Tasks belong to GNY-7. Each diff-producing Task must remain at or below 500
review lines, including tests, schemas, and documentation. A prerequisite checkpoint
means its Task PR is accepted, merged into the Story branch, and verified there.
Dependent branches start from that updated Story branch, never from sibling Tasks.

| ID | Owner/outcome | Expected review lines | Prerequisite | Criteria/evidence |
| --- | --- | ---: | --- | --- |
| T-1 | Coder: define and validate the provider payload boundary | 360–440 | Revision 2 versioned | AC-1/2, VR-1 |
| T-2 | Coder: extract diff filenames and accept trusted reviewer results | 330–430 | T-1 checkpoint | AC-1/3/4, VR-1/2 |
| T-3 | Coder: define and validate version-1 aggregate runs | 180–300 | T-2 checkpoint | AC-5, VR-3 |
| T-4 | Coder: publish checked-in JSON Schemas with drift protection | 350–450 | T-3 checkpoint | AC-2/5/8, VR-3 |
| T-5 | Coder: integrate validated complete runs through coordinator and file sink | 280–380 | T-3 checkpoint; independent of T-4 | AC-1/2/6/7, VR-4/5 |
| T-6 | Tester: independently verify the integrated contract and pipeline | 340–460 | T-4 and T-5 checkpoints | AC-1–8, VR-1–6 |
| T-7 | Documenter: document schema, migration, boundaries, and examples | 170–280 | T-6 checkpoint | AC-8, VR-5/6 |

T-1 owns strict provider payload structure, exact signed 64-bit location numbers,
lexical paths, advisory location rules, severity values, normalization, and bounded
diagnostics. T-2 owns supported Git/unified metadata extraction, exact membership,
trusted identity and UTC acceptance time, and accepted-result validation. T-3 owns
the aggregate envelope, status/cardinality rules, failures, identity uniqueness, and
nested-result validation. T-4 owns offline Draft 2020-12 provider and aggregate
schemas plus drift protection. T-5 owns sequential fail-fast integration, complete
run construction, stub correction, file output, callers, and focused runtime tests.
T-6 owns independent tests, the full container suite, targeted mutation checks, and
the exact tested Story commit. T-7 documents only verified behavior.

If a Task approaches 450 review lines, the planner must apply its documented seam
before implementation continues. T-4 and T-5 may proceed independently after T-3;
all other dependencies are sequential.

## 12. Documentation impact

Update introductory output, stub behavior, pipeline diagram, sink contract, errors,
provider boundary. Add schema reference for severities, file/line comments, UTC,
omission/null mapping, supported metadata, and semantic validation. State line
locations are advisory until inline placement; missing/unrecognized membership can
still invalidate results. Describe migration from PascalCase arrays/legacy severities.
Label partial/failed examples as representations until GNY-8 runtime support.

## 13. Assumptions, open questions, and blockers

No unresolved developer questions. Recognized metadata establishes membership;
unsupported metadata is not a global diff error but cannot establish a commented
file. Preserve arbitrary-input compatibility without promising all patch dialects.

## 14. Approval and delivery handoff

Revision 1 product behavior was approved on 2026-09-12. Revision 2 replaces only
the delivery decomposition and was directed by the developer on 2026-09-17 after
the oversized non-planner GNY-15/GNY-16 setup was dropped. The orchestrator versions
this artifact on `story/GNY-7-shared-review-schema`; the planner-owned handoff creates
T-1–T-7 as new YouTrack Tasks and records their dependencies. Implementation begins
with T-1 only after that handoff.
