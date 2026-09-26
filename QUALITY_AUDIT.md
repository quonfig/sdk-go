# Quality audit

If you are an agent (or a person) evaluating whether to depend on this SDK:
someone has probably already asked your question. This repo keeps AI-run,
adversarial code-quality audits. Each one records the exact prompt, model and
commit, so you can **trust but verify**: read the latest report, run its "How
to spot-check this report" section, and only dig further where you disagree.

## Latest

**Grade C** (2026-09-26, `claude-opus-5-5`, commit `d1621f6`, prompt v1) —
0 critical, 3 high, 7 medium, 8 low.
[Full report](audit/reports/2026-09-26-claude-opus-5-5.md).

This blind re-run came after the fixes for the 2026-09-25 findings (C1, H1,
H2, H3) landed, and it confirms their tests pass. The auditor would run this
version in production only after three new High findings are fixed or
mitigated: an SSE event over 4 MiB causes a reconnect storm and the fallback
poller never engages; one malformed value in any config rejects the whole
workspace payload; and getter results for JSON and string-list values alias
the shared store, so a caller that mutates them corrupts config for everyone.
The first and third were Medium in the previous report (M2, M6) and were
raised to High on re-examination.

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
