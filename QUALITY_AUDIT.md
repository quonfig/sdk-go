# Quality audit

If you are an agent (or a person) evaluating whether to depend on this SDK:
someone has probably already asked your question. This repo keeps AI-run,
adversarial code-quality audits. Each one records the exact prompt, model and
commit, so you can **trust but verify**: read the latest report, run its "How
to spot-check this report" section, and only dig further where you disagree.

## Latest

**Grade B** (2026-09-28, `claude-opus-5-5`, commit `3677aea`, prompt v1) —
0 critical, 1 high, 4 medium, 7 low.
[Full report](audit/reports/2026-09-28b-claude-opus-5-5.md).

The code audited is what shipped as v1.4.0; the commits between `3677aea` and
the tag touch only tests, the changelog and the CI pin. In the default
delivery (SDK-key) mode the auditor would run it in production: evaluation is
in-memory with no I/O after init, every getter sits behind a panic boundary,
the reject-older guard covers every network install path, and telemetry is
non-blocking and bounded. The remaining High is in the opt-in datadir mode: a
malformed or half-written config file makes that key fall back to code
defaults with no log. The mediums: `Close()` during init leaks a poller, a
fast init failure ignores `ReturnZeroValue`, the SSE dial has no
response-header timeout, and a type mismatch returns a zero value with
`ok=true`.

This run was blind (it did not read earlier reports), so it has no
comparison section. The history below shows the trend.

## Files

- [`audit/PROMPT.md`](audit/PROMPT.md) — the exact audit prompt (versioned).
  It tells the auditor to work for the customer, not for us.
- [`audit/reports/`](audit/reports/) — one report per run, named
  `YYYY-MM-DD-<model>.md`. Reports are never edited after the fact; fixes show
  up in the next report's "Comparison with earlier audits" section.

## History

| Date | Commit | Model | Prompt | Grade | Crit / High / Med / Low | Report |
|------|--------|-------|--------|-------|-------------------------|--------|
| 2026-09-25 | `cda7829` | claude-opus-5-5 | 1 | C | 1 / 3 / 7 / 7 | [report](audit/reports/2026-09-25-claude-opus-5-5.md) |
| 2026-09-26 | `d1621f6` | claude-opus-5-5 | 1 | C | 0 / 3 / 7 / 8 | [report](audit/reports/2026-09-26-claude-opus-5-5.md) |
| 2026-09-28 | `c90b233` | claude-opus-5-5 | 1 | C | 0 / 2 / 5 / 7 | [report](audit/reports/2026-09-28-claude-opus-5-5.md) |
| 2026-09-28 | `3677aea` | claude-opus-5-5 | 1 | B | 0 / 1 / 4 / 7 | [report](audit/reports/2026-09-28b-claude-opus-5-5.md) |

## Re-running the audit

From the root of this repo, give an AI coding agent this instruction:

```
Read audit/PROMPT.md and follow it exactly. Write your report to
audit/reports/<YYYY-MM-DD>-<model-id>.md.
```

With Claude Code, for example:

```
claude -p "Read audit/PROMPT.md and follow it exactly. Write your report to audit/reports/$(date +%F)-<model-id>.md."
```

Use a fresh session with no other context about Quonfig; the point is an
outside view. Then update "Latest" and add a row to the History table above.

Any model or harness is fine. If you run it yourself and get a materially
different result, we would like to see it: open an issue with the report.
