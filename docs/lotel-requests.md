# lotel Request Directory

Cross-repo requests for `lotel` should be written as markdown files under:

```text
/home/infra-admin/git/lotel/.agents/requests/
```

The directory is intended for agent-authored request notes that a human or a
`lotel` maintainer can review before making source changes in the Rust repo.
Do not modify Rust source in `/home/infra-admin/git/lotel` when filing a
request from this repo.

## Verification

Verified on 2026-05-25 from this repository:

```bash
ls -la /home/infra-admin/git/lotel/.agents /home/infra-admin/git/lotel/.agents/requests
```

Result at initial verification: `/home/infra-admin/git/lotel/.agents/requests/`
did not exist, so read/write checks against the request directory could not
pass yet.

The `lotel` checkout already had unrelated local changes at verification time:

```text
 M build.sh
?? .claude/
?? lotel
```

No files in `/home/infra-admin/git/lotel` were changed for this verification.

## Filed Requests

### GenAI Query Support

Filed on 2026-05-25:

```text
/home/infra-admin/git/lotel/.agents/requests/genai-query-support.md
```

Request summary:

- Add a GenAI-aware query affordance for current `gen_ai.*` attributes emitted
  by `agent-otel`.
- Promote model-call fields such as `gen_ai.operation.name`,
  `gen_ai.provider.name`, `gen_ai.request.model`,
  `gen_ai.response.model`, `gen_ai.usage.input_tokens`, and
  `gen_ai.usage.output_tokens` into a stable query surface.
- Include shared fallback and availability keys:
  `agent_otel.usage.available`, `agent_otel.provider.from`, and
  `agent_otel.provider.to`.
- Keep payload fields such as `gen_ai.input.messages` out of the first query
  surface because payload capture is opt-in and redaction-sensitive.

## Creation Step

When a `lotel` maintainer is ready to accept cross-repo agent requests, create
the request directory in the `lotel` repository:

```bash
mkdir -p /home/infra-admin/git/lotel/.agents/requests
```

After creating it, verify access with a temporary probe file:

```bash
printf '%s\n' '# write-check' > /home/infra-admin/git/lotel/.agents/requests/.write-check.md
test -r /home/infra-admin/git/lotel/.agents/requests/.write-check.md
rm -f /home/infra-admin/git/lotel/.agents/requests/.write-check.md
```

## Request Format

Use one markdown file per request. Name files with a sortable date prefix and a
short kebab-case summary:

```text
YYYY-MM-DD-short-request-title.md
```

Use this structure:

```markdown
# Request: Short actionable title

## Source

- Repository: agent-otel
- Bead: agent-otel-<id>
- Requested by: <agent or person>
- Date: YYYY-MM-DD

## Context

Briefly describe why the request exists and what upstream `agent-otel` work it
unblocks.

## Request

Describe the exact `lotel` behavior, API, fixture, or verification support
needed.

## Acceptance Criteria

- Observable result that proves the request is complete.
- Any compatibility or non-goals the maintainer should preserve.

## Notes

Optional links, traces, examples, or constraints.
```
