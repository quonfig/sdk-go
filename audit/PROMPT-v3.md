> PROMPT v3, copied from canonical private copy on 2026-10-27 at project c876c18 (`project/sdk-quality-check/PROMPT.md`). Paths under `project/` below refer to that private repo; in this repo, earlier reports live in `audit/reports/`.

# Independent SDK Code-Quality Audit: Prompt

Prompt version: **3** (2026-10-22). This is the one canonical copy;
`sdk-go/audit/PROMPT.md` is v1, kept as history. If you change this prompt,
bump the version. Every report records the version it used in its front
matter, so results can be compared across runs.

v1 (`sdk-go/audit/PROMPT.md`, 2026-09-25) was written for Go. Every auditor of
the other nine SDKs adapted it by hand. v2 (2026-10-07) is language-neutral,
adds an SDK class with fixed N/A rules, derives the grade from a rubric, adds a
chaos policy, makes two report sections required, and adds a cross-SDK hazard
checklist that you read only after your blind pass. Background:
`project/sdk-quality-check/META-ANALYSIS.md` section 5.

v3 (2026-10-22, bead qfg-6vno.19) fixes the rubric issues found in the
2026-10-16 re-audit (`STATUS-2026-10-16.md`, "Rubric issues for PROMPT v3"):
- S1 judges only releases since the previous audit **and** since Decision 1
  (2026-10-05). A release the Decisions log approved is a process note, and a
  refutation is reported once, not carried forever.
- S1 Verified needs an "approved as minor under Decision 1 (<bead id>)"
  CHANGELOG line on every who-matches or common-path change shipped as a
  minor. An approval recorded only in beads is a Low.
- A severity calibration table ("Severity calibration") gives the same finding
  class the same severity in every SDK. Any deviation needs one line of
  justification.
- A behavior that matches a recorded product or design decision is not a
  finding.
- The write-time regex check (qfg-ktkr) settles the SDK-side ReDoS severity.

---

## Your role

You are an independent auditor. A prospective customer is deciding whether to
depend on this SDK in production. They will ask you one question:

> **How good is this code, and what could it do to my production service?**

You work for them, not for the vendor. The vendor (Quonfig) wrote the briefing
below. Treat every statement in it as a claim to check, not a fact. Where you
can verify a claim, do so and say how. Where you can't, say so. Where it is
false, say that plainly.

Be adversarial but fair. A finding needs evidence: a file and line, a command
you ran and its output, or a small reproduction. Don't pad the report with
style nits to look thorough, and don't soften real problems because the rest
of the code is good. "I looked for X and found nothing" is a useful result;
say what you looked at.

Do not change any source files in the repo you audit. Only write the report.

## SDK class

Decide the class first. Put it in the front matter as `sdk_class`.

- **backend**: evaluates rules locally in the customer's process. Today: sdk-go,
  sdk-node, sdk-ruby, sdk-python, sdk-net, sdk-java.
- **frontend**: the server evaluates rules for one context, and the SDK holds
  the results. Today: sdk-javascript, sdk-swift.
- **wrapper**: framework bindings or a platform shim over another Quonfig SDK.
  Today: sdk-react (over `@quonfig/javascript`) and sdk-react-native (over
  `@quonfig/react`). OpenFeature providers would also be wrappers.

If the repo doesn't match its listed class, say why in "Adaptations to
prompt" and audit it as the class it really is.

**Wrappers.** A customer who installs the wrapper gets its upstream SDK at
whatever versions the wrapper's dependency and peer ranges resolve to today.
Audit both:
- the wrapper's own source, in the repo;
- the upstream code, from the **published** packages those ranges resolve to
  (for example `npm pack <pkg>@<version>` into scratch), not from the upstream
  repo's HEAD.

List the resolved upstream versions in the `upstream` front-matter field.
Upstream defects count as findings in this report, because the customer gets
them. Mark each one `Inherited from: <package>@<version>`, so the cross-SDK
analysis can tell them apart from the wrapper's own defects.

## Vendor briefing (claims to verify)

**What the SDK does, by class.**

- **backend.** Downloads the full config set for one environment from the
  delivery service (`api-delivery`), keeps it in memory, and evaluates rules
  locally, so a get/enabled call makes no network request. Updates arrive over
  SSE. There is a primary and a secondary delivery host, a fallback poller for
  when the stream is unhealthy, and a "datadir" mode that reads config from a
  local directory (a git checkout) instead of the network. Background
  telemetry sends evaluation counts, context shapes and example contexts.
- **frontend.** Sends the current context to `api-delivery`
  (`eval-with-context`), receives the server-evaluated values for that
  context, and serves reads from memory. HTTP polling keeps the values fresh,
  and the primary and secondary hosts are hedged. A last-known-good cache
  persists values where the platform has storage. There is no SSE, no datadir
  and no local rule evaluation. Telemetry sends evaluation summaries. Context
  shapes and examples are recorded server-side from the fetch.
- **wrapper.** Exposes the upstream SDK through the framework's idioms
  (provider, hooks, test helpers) or makes it run on the platform (polyfills).
  Transport, evaluation and telemetry are the upstream SDK's.

**Stability claims.** These four are the claims the grade rubric uses.
- **S1.** Releases follow the project's semver policy. Within the same major
  line (1.x, 2.x), no minor or patch release made an incompatible change to
  the public API: no public export was removed or renamed, and none changed so
  that caller code that built and ran against the release before it no longer
  does. Backward-compatible additions (a new export, a new optional parameter
  or field, a widened type) are allowed. A fix that changes behavior only on a
  rare edge (type mismatch, malformed value, error path) ships in a minor; a
  fix that only changes error-path messages may ship in a patch. When the
  release's CHANGELOG entry describes such a change in a minor, it does not
  refute S1. Three things do refute it: a patch that changes edge behavior
  beyond error-path messages, even when its CHANGELOG describes the change
  (Decision 1 requires a minor); a minor or patch that changes a result on a
  common success path (for example, the value a typed getter returns for a
  flag of the matching type, or which users a targeting rule matches), unless
  the approval is recorded (see "Approval line" below); and a behavior change
  the CHANGELOG does not describe.

  **Window.** Judge only the releases that are newer than **both** of these:
  1. the commit your previous audit of this SDK recorded (the `commit` field
     of the earlier report's front matter; read only that field now);
  2. the Decision 1 date, **2026-10-05** (the semver policy above dates from
     then; v2 audits started on 2026-10-07).
  If there is no earlier audit, the window starts at the Decision 1 date.
  Releases before the window were judged by earlier audits or predate the
  policy. Do not judge them again.

  **Approved releases.** The Decisions log
  (`project/sdk-quality-check/META-ANALYSIS.md`, "Decisions log") records
  release trains that Jeff approved release by release (for example, Decision
  9 on 2026-10-09 approved sdk-ruby 1.7.2 and sdk-python 1.7.1). Read only the
  log entries that name this SDK's releases; do not read the rest of the file
  until step 8. If a release in the window would refute S1 but the log
  explicitly approved that release, it does not refute S1. Record it as a
  process note in the S1 evidence ("1.7.2 changes edge behavior in a patch;
  approved in Decision 9"), and do not count it as a finding.

  **Report a refutation once.** A published release never changes. If an
  earlier audit already reported a release as refuting S1, that release is
  outside your window. Mention it in "Comparison with earlier audits" as a
  historical note. It does not set this audit's S1 verdict.

  **Approval line.** A who-matches or common-path change shipped in a minor
  counts as approved only when its CHANGELOG entry carries a line of the form
  `approved as minor under Decision 1 (<bead id>)`, for example
  `approved as minor under Decision 1 (qfg-goi1.5)`. Then S1 can be Verified
  for that change. If the approval is recorded only in a bead or the Decisions
  log, and the CHANGELOG line is missing, S1 is still Verified for that change,
  and the missing line is a **Low** (API design: customers reading the
  CHANGELOG cannot see that the change was deliberate). If no approval is
  recorded anywhere, S1 is Refuted. To check a cited bead, `bd show <id>` if
  `bd` is available; otherwise say you could not check it and mark S1 Not
  verified for that change.

  Check it by diffing the public API of each release tag in the window against
  the release before it, and by reading each release's CHANGELOG entry.
  Unreleased changes on HEAD are not releases: report problems in them as
  findings, not under S1. On a 0.x line semver permits breaking changes, so S1
  is N/A (see the class table).
- **S2.** Once the SDK has initialized, a network failure or a delivery
  outage never makes evaluation fail or block. The worst case is serving the
  last known values. Startup with delivery unreachable is governed by the
  init-failure policy (`project/plans/2026-10-05-init-failure-unification.md`:
  server SDKs fail init by default, client SDKs continue on cache then
  defaults), so refusing to boot does not refute S2. A client object that
  exists but can never recover (dead getters, or defaults forever with no
  retry) is a Resilience finding; it refutes S2 only if an already-initialized
  client stops evaluating.
- **S3.** Updates are ordered: the SDK never replaces a newer config with an
  older one (a "generation" number guards this). Frontend SDKs guard within one
  context. A context switch is its own install path, and it must never install
  another context's values.
- **S4.** Telemetry is best-effort and bounded in memory. It never blocks or
  crashes the host application.

Paying customers run these SDKs in production. That is context for rating
severity, not a claim to verify.

**Known intentional behaviors.** These look like bugs but are decisions. Check
that the code matches the decision, and say if you think the decision itself
is wrong.
- **I1.** Weighted rollouts normalize by the total weight. Weights that don't
  sum to 100000 still work (for example, 100000/80000 serves about
  55.6%/44.4%).
- **I2.** A payload with the same generation as the one held is dropped
  silently. Only a strictly older one counts as a "guard rejected" event. Some
  SDKs emit no such event; if this one doesn't, say so.
- **I3.** Telemetry batches that fail to send are kept and resent unmerged,
  with caps on count, bytes and age. The oldest are dropped first.
- **I4.** The chaos workflows check out `api-delivery` at `main`, not at a
  pinned SHA. This is deliberate: the nightly chaos runs exist to catch
  api-delivery drift. Using main is not a finding. A chaos run that does not
  log the api-delivery SHA it tested is (Decision 8).

**Recorded decisions are not findings.** I1 to I4 are not the only decisions.
The Decisions log (`META-ANALYSIS.md`, "Decisions log") and closed decision
beads record more product and design choices. Examples: the C16 privacy
defaults (example contexts and dev-context injection on by default); the
init-failure policy; the swift privacy-first store reset (Decision 9). When the
code matches a recorded decision, do not count it as a finding, even if you
would have decided otherwise. Instead:
- list it in the Vendor claims table after I4, as a row `D: <decision>` with
  verdict Verified (the code matches) or Refuted (it does not), and the
  decision's source (log entry number or bead id);
- if you think the decision is wrong, say so in the Verdict, with the
  production risk. That is input for Jeff, not a counted finding.

Code that does **not** match the recorded decision is a normal finding. A
decision that is still open (a decision bead with status open or blocked) is
not recorded yet: rate the behavior as a finding and cite the open bead. For
example, the forked worker that discards the parent's snapshot (python, ruby)
is a finding until qfg-6vno.16 records its decision.

**How correctness is tested across SDKs.** The shared behavior contract lives
in a separate repo, `quonfig/integration-test-data` (ITD). Cross-SDK behavior
is defined there: where ITD covers a behavior, it is the reference.
- `tests/eval/*.yaml`: language-neutral cases for config lookup, flag
  evaluation, targeting rules, weighted rollouts, context precedence, error
  behavior and telemetry payloads.
- `data/integration-tests/`: the config, flag and segment JSON those cases run
  against.
- `generators/`: a code generator that turns the YAML into idiomatic tests for
  each backend SDK. The output is committed in each repo. CI regenerates the
  tests at a pinned ITD tag and fails on any diff.
- `tests/duration/grammar.yaml`: the duration grammar, which some frontend SDKs
  also consume.
- `chaos/`: a network-fault harness (toxiproxy) with scenarios for dropped
  streams, slow responses, failover between primary and secondary, and
  out-of-order delivery. It also holds contract documents, such as the
  telemetry transport contract.

**What applies to each class.** "N/A" means: mark it N/A in the report, it
does not count in the grade rubric, and you don't need to explain it. If
anything else is not applicable, explain why in "Adaptations to prompt".

| Briefing item | backend | frontend | wrapper |
|---|---|---|---|
| S1 semver | applies (N/A on a 0.x line) | applies (N/A on a 0.x line) | applies (N/A on a 0.x line) |
| S2 network failure never fails or blocks evaluation | applies | applies | applies (through upstream) |
| S3 ordering | applies | applies (within a context, plus the context-switch rule) | applies (through upstream) |
| S4 telemetry bounded, never blocks or crashes | applies | applies | applies (through upstream) |
| I1 weighted normalization | applies | N/A (server evaluates) | N/A |
| I2 same-generation drop | applies | applies | applies (through upstream) |
| I3 telemetry resend | applies | applies | applies (through upstream) |
| I4 chaos uses api-delivery `main` | applies | N/A | N/A |
| SSE, fallback poller, datadir mode | applies | N/A | N/A |
| ITD eval corpus, generator and regen-drift check | applies | N/A, except the duration grammar if the repo consumes it | N/A, unless the repo consumes ITD |
| Chaos rigs | applies | N/A unless the repo has one | N/A unless the repo has one |

## What to do

Work in this order and keep notes of every command you run. Steps 1 to 7 are
your **blind pass**. Don't read anything that step 1 lists until step 8 or
step 9 tells you to.

### 1. Form your own view first

During the blind pass, **do not read**:
- this SDK's earlier reports, wherever they live (the repo's `audit/reports/`
  and `QUALITY_AUDIT.md`, or `project/sdk-quality-check/reports/<this-sdk>-*.md`
  for the 2026-10-05 first round), except the front-matter `commit` field that
  S1 needs: open the newest earlier report, read that one field, close it;
- anything else in `project/sdk-quality-check/` other than this PROMPT.md (the
  meta-analysis, other SDKs' reports, the `yagni/` files). Two exceptions:
  for S1, the Decisions log entries that name this SDK's releases; and for
  "Recorded decisions" below, the Decisions log itself. Read the log
  (`META-ANALYSIS.md`, "Decisions log" section) only, not the clusters or the
  scoreboard;
- other SDKs' audit reports.

Earlier conclusions anchor you. You may read other SDKs' **source code** at
any time; step 7's reference comparison requires it.

### 2. Build and run the tests

Record the commit SHA (`git rev-parse HEAD`), the latest tag
(`git describe --tags`) and the toolchain versions.

Take the commands from the repo's CI workflows (`.github/workflows/*`), and
run what CI runs:
- build;
- format and lint checks;
- type check, where the language has one;
- the unit and integration tests;
- a race detector or sanitizer, if the ecosystem has one (for example TSan).
  If it doesn't, judge concurrency by reading the code and by targeted
  interleaving repros;
- the vulnerability or dependency scan CI runs. If CI runs none, use the
  ecosystem's standard scanner if it is installed, and say if it isn't;
- for backend SDKs, the ITD regeneration and diff, the way CI does it. Clone
  `integration-test-data` beside your copy and check out the tag CI pins. If
  you can't regenerate, say you skipped this.

If a CI command can't run locally (missing secrets, OS, a runtime version),
run the closest equivalent and say what you substituted. CI usually tests a
matrix of runtime versions; say which ones you ran.

Report pass/fail counts and anything flaky (rerun the suite at least twice).
A failing test is a finding. If the suite skips something that matters, say
what.

### 3. Judge the test suite, not just its result

Green tests prove only what they test. Look for:
- **Contract coverage (backend).** Pick at least five behaviors from
  `tests/eval/*.yaml`. Confirm that the generated test really asserts each one
  (not skipped, not asserting a weaker condition). Then look for SDK behaviors
  the shared suite does not cover at all.
- **Hand-written coverage (frontend and wrapper).** Without a generated
  corpus, judge whether the repo's own tests exercise the real stack or a stub,
  and whether the behaviors behind the S2 and S3 claims are tested at all.
- **Tests that can't fail:** assertions only on mocks, skips without a reason,
  timing-based tests that pass by luck, and setup that short-circuits the code
  the test claims to test.
- **The gap between ITD and this SDK:** generated cases that are skipped,
  overridden or disabled for this language.

### 4. Chaos

Backend repos have chaos workflows (`chaos.yaml`, `failover-chaos.yaml`) that
drive the ITD `chaos/` scenarios against a real `api-delivery` through
toxiproxy. Frontend and wrapper repos have none today, so this step is N/A
for them unless the repo has its own rig.

**Run the chaos rigs only if you are isolated:** Docker is already running,
you work in your own copy, and nothing else (another auditor, a dev server)
can use the same ports. Otherwise, don't run them. Read the scenarios and the
harness code that drives them, judge what they would and would not catch, and
say in "What I ran" that you did not run them, and why. If `gh` is available,
check a recent chaos run of the workflow with `gh run view` and say whether it
really ran the scenarios or took a skip path (its duration is a quick tell).
Record the result in the `chaos` front-matter field.

Whether or not you run them, make these two required checks:
1. **The chaos pin equals the unit pin.** Compare the ITD `ref:` that the
   chaos workflows check out with the one the unit-test workflow checks out.
   If they differ, it is a finding: diff `chaos/` between the two tags and rate
   it by what the stale pin misses.
2. **No stubbed expressions.** Read how the harness evaluates each scenario
   expectation and metric. Every expression or metric name the harness does
   not implement must be reported as an explicit SKIPPED with a reason in the
   run output. If it silently returns a constant (0, true, an empty value), or
   an unknown name falls through to a default instead of failing, that is a
   finding. List each stub with its file:line and the scenarios that use it. An
   explicit SKIPPED with a stated reason is acceptable and not a finding.

### 5. Read the code for production risk

Prioritize what a customer's on-call engineer cares about. For each area, say
what you checked and what you found. Translate each item into your language's
terms, and record the translation in "Adaptations to prompt".

1. **Hot path.** get/enabled/evaluation: locking, allocations, anything that
   can block, throw or do I/O. Can a malformed config or context crash the
   caller: a panic or exception escaping into the caller's thread, request,
   render or event loop, or an unhandled promise rejection?
2. **Concurrency and lifecycle.** Shared state, lock ordering, and every
   thread, task, timer, listener and socket the SDK starts. Does the shutdown
   call stop all of them? Any leak on repeated create and shutdown? Any race
   that no test exercises?
3. **Network failure.** Reconnect and backoff, failover between hosts, fallback
   polling, timeouts on every HTTP call, and behavior when the server sends
   garbage or a huge body. Does init block, and for how long?
4. **Ordering.** Can an older config ever replace a newer one (reconnect,
   failover, poller vs stream, context switch)?
5. **Telemetry.** Memory bounds, blocking behavior, what it sends, and
   whether any of it is sensitive (context attribute values, secrets).
6. **Secrets and security.** Encrypted config values, API key handling,
   logging of keys or values, TLS settings, and path handling in datadir mode.
7. **API design.** Is the public API idiomatic for the language, hard to
   misuse, and documented? Are errors returned, raised or swallowed? What
   happens on a type mismatch?
8. **Dependencies.** What is declared, how old, any known advisories, and
   anything declared but unused.
9. **Maintainability.** Size and structure, duplication, dead code, comment
   quality, and how hard it would be for someone outside Quonfig to fix a bug.

You don't need to read every line. Spend your time where a defect would hurt a
customer most, and say what you did not look at.

### 6. Verify the vendor claims

For S1 to S4, I1 to I4, and each recorded decision you checked (`D:` rows),
give one verdict from this list:
**Verified**, **Partly verified**, **Not verified** (couldn't check),
**Refuted**, or **N/A** (only where the class table says so). Then give the
evidence. If the honest answer is mixed ("verified for steady state, refuted
for init"), say so in the evidence, but the verdict column holds the **worse**
of the two. The rubric reads the verdict column.

### 7. Reference comparison (for any divergence claim)

When a finding says this SDK **diverges** from expected cross-SDK behavior,
cite the reference you compared against, with file:line. Use, in order:
1. the ITD case that pins the behavior, if one exists;
2. the canonical implementation, which is sdk-go unless the repo or ITD
   declares another one.

Read that reference code; don't assume it. If neither ITD nor the reference
pins the behavior, say "unspecified", not "divergent". Then the finding is
about the missing contract, and it belongs under "Likely cross-SDK issues".

**This ends the blind pass.** Finish your findings, claim verdicts and grade
inputs before you go on.

### 8. Cross-SDK hazard checklist

Only now, read section 2 ("Cross-SDK clusters") of
`project/sdk-quality-check/META-ANALYSIS.md`. It is the maintained list of
hazards that earlier audits found across several SDKs. For each cluster (C1 and
up), check whether it applies to this SDK, and record a row in the checklist
table: **checked, present** (link the finding, or add one), **checked, absent**
(say what you read), **N/A** for this class, or **unchecked** (say why). If you
can't read that file, say so and skip this step.

A hazard you find here is a normal finding. Mark it `Found in: step 8`, so a
reader can tell blind findings from checklist findings.

### 9. Compare with earlier reports

Now read this SDK's earlier reports, if any (`audit/reports/` in the repo,
and `project/sdk-quality-check/reports/<this-sdk>-*.md`). For each earlier finding, say
whether it is fixed (with the commit if you can find it), still present, or
was wrong. Note anything you found that they missed, and anything they found
that you missed (and whether you agree after checking).

## Severity scale

- **Critical**: can take down or corrupt the customer's service. Examples: a
  crash in the caller's thread, request or render; a deadlock; unbounded
  memory; silently serving wrong values; leaking secrets.
- **High**: likely to cause an incident under realistic failure conditions
  (network partition, delivery outage, deploy of the customer's app).
- **Medium**: a real defect or risk, but it needs an unusual condition or has
  a limited blast radius.
- **Low**: code quality, docs or test gaps with no direct production effect.

Tag every finding with exactly one scorecard **Area**: Correctness,
Resilience, Concurrency, Security, Test suite, API design or Maintainability.
The rubric uses the tag.

### Severity calibration

The same class of finding must get the same severity in every SDK. Without
this, the 2026-10-16 audits rated "failed init never retried" High in one
wrapper and Medium in its upstream, and the grades could not be compared.
When a finding belongs to a class below, use the baseline severity and area,
and move it one level only when a listed condition holds. The cluster IDs
(C1 and up) are those of `META-ANALYSIS.md` section 2; you don't need to read
that file to use this table (step 8 still waits for the end of the blind
pass).

A bead's priority (P1, P2, ...) is a scheduling choice, not a severity. Do
not use it to set or adjust a severity.

| Class | Baseline | Raise one level when | Lower one level when |
|---|---|---|---|
| C1 Secret or confidential value (decrypted, `confidential`, ENV_VAR) in error text, logs or exceptions | High, Security | it leaves the host by default (telemetry, OpenFeature error events): Critical | the value is not confidential and appears only in local error text: Low (lower by two) |
| C2 Backend: client object exists after a caught init failure but never recovers (getters throw or defaults forever, no retry) | High, Resilience | - | the dead state needs a non-default option: Medium |
| C2 Frontend or wrapper: failed init never self-retries (defaults or cache for the session, polling never starts) | Medium, Resilience | getters throw or the render crashes instead of serving defaults: High | - |
| C2 Refusing to boot when delivery is unreachable, per the init-failure policy | not a finding | - | - |
| C3 Typed getter on a type mismatch, resolution error or missing key: wrong value, or reports found/ok, or ignores a caller-supplied default, or throws although a default was supplied | Medium, Correctness | the failure cannot be caught by the caller (trap, process abort, uncatchable exception): High | the path needs a malformed value no writer can store: Low |
| C3 Boolean helper coerces a string (`"false"` is true) | Medium, Correctness | - | - |
| Wrong value or type for a config shape that the docs and ITD support, returned as success (for example weighted json or provided variants returned as `toString()`) | High, Correctness | - | the shape is undocumented and absent from ITD: Medium |
| C4 Transport health: tight reconnect loop (more than about 1 per second) or the fallback poller never engages | High, Resilience | - | - |
| C4 Transport health: no response-header timeout, backoff cap or delay that departs from the contract, missing WARN on a bad key or init failure | Medium, Resilience | - | only the log level is wrong: Low |
| C5 Another user's or context's values installed or rendered | High, Correctness | across requests in SSR, or persisted under the new identity: Critical | it needs a non-default API: Medium |
| C6 Telemetry over caller context values: one bad attribute drops the whole window | Medium, Resilience | the process crashes, or memory grows per evaluation without a bound: High | - |
| C7 Operator semantics differ from the ITD case or sdk-go | Medium, Correctness | it changes who matches on a common value (not a boundary or a rare type): High | - |
| C8 `PROP_MATCHES` ReDoS or catastrophic backtracking in the SDK | Low, Security (see "Regex and write-time enforcement" below) | - | - |
| C8 Regex compiled on every evaluation (cost only) | Low, Maintainability | - | - |
| C9 One malformed row or value rejects the whole payload | Medium, Resilience | init hangs, or updates wedge until restart: High | - |
| C10 Env-scoped `decryptWith` key evaluated in the wrong environment | High, Security | - | - |
| C11 Returned JSON or list value aliases the cache (backend) | Medium, Correctness | - | frontend or wrapper (one user per process): Low |
| C12 Lifecycle: double init, close during init, leaked thread, socket or timer per create and close | Medium, Concurrency | - | it needs close to race init by a narrow window (TOCTOU): Low |
| C13 No size cap on an SSE event or config body | Medium, Resilience | - | - |
| C13 Busy-poll, or storage that grows without eviction | High, Resilience | - | - |
| C14 Chaos probe returns a constant, or an unknown expression falls through to a default, instead of an explicit SKIPPED | Low, Test suite | it hides a real Resilience defect this audit found: Medium | - |
| C14 Chaos ITD pin differs from the unit pin | Low, Test suite | the diff between the two tags contains a scenario the stale pin misses: Medium | - |
| C15 Semver | judged under S1, not as a finding. The one exception: a missing approval line is a Low (see S1) | - | - |
| C16 Privacy defaults | not a finding while the code matches the recorded decision (see "Recorded decisions are not findings") | - | - |
| C17 React Native: crash at import or first use (missing `TextEncoder`, module-load side effect) | High, Resilience | - | - |
| C17 React Native: packaging trap (native dependency not a peer, a peer floor that admits a broken upstream) | Medium, API design | - | - |

Classes not in the table: use the severity scale above.

**Deviations.** If you rate a listed class differently, add one line to the
finding: `Calibration deviation: <baseline> -> <your rating>, because <reason>`.
The reason must be about this SDK's code or blast radius, not about how
another audit rated it. A documented workaround in the README does not lower
a severity: customers hit the default path.

### Regex and write-time enforcement

Decision 4 makes the fix for regex ReDoS (C8) a write-time check, not an SDK
change. The check is Quonfig regex v1 (epic qfg-ktkr, plan
`project/plans/2026-10-12-regex-write-time-check.md`). It rejects any
`PROP_MATCHES` or `PROP_DOES_NOT_MATCH` pattern outside a safe RE2 subset in
`qfg verify`, the app, the API, and the app-gitea pre-receive hook.

- **Once the app-gitea hook rejects non-v1 patterns in production**
  (qfg-ktkr.4 closed), the SDK-side ReDoS is cleared for stored rules. It is
  not a finding. Every pattern an SDK can receive has passed the hook. If the
  SDK has no runtime regex timeout, you may still note that as a Low
  (Security, defense in depth), not as a Medium.
- **Until then**, rate SDK-side ReDoS **Low** (Security), not Medium. Cite
  Decision 4, and the 2026-10-12 prod corpus scan (zero stored regex criteria
  in 30 workspaces), which the plan records. Check the status with
  `bd show qfg-ktkr.4` if `bd` is available, and say which case applied.
- Either way, if the SDK evaluates a v1 pattern differently from sdk-go
  (`tests/regex/dialect.yaml` in ITD pins the dialect), that is a C7-style
  Correctness finding, not a ReDoS finding.

## Grade rubric

The overall grade is computed, not chosen. Its inputs are:
- the finding counts by severity;
- the Medium findings tagged Resilience or Security;
- the verdicts for the four stability claims S1 to S4 (N/A claims are left
  out);
- the lowest grade in your scorecard. Grade each scorecard area on the scale
  A, A-, B+, B, B-, C, D, F. The letters mean what the overall grades below
  mean; a plus or minus sits between two neighboring letters.

If the audited commit does not build, the grade is F, regardless of the other
rows. Otherwise, the grade is the **highest** row whose conditions all hold:

| Grade | Critical | High | Medium | Claims S1-S4 (applicable ones) | Scorecard |
|---|---|---|---|---|---|
| **A** | 0 | 0 | at most 1, none in Resilience or Security | all Verified | no area below B+ |
| **B+** | 0 | 0 | at most 2, none in Resilience or Security | all Verified | no area below B- |
| **B** | 0 | at most 2 | any | none Refuted | any |
| **C** | at most 1 | any | any | any | any |
| **D** | at most 2 | any | any | any | any |
| **F** | 3 or more | | | | |

Meaning: **A**: you'd depend on it without reservations. **B+**: the program
target; no open High risk and no Medium in the areas that page on-call.
**B**: solid, with issues worth tracking. **C**: usable, but fix the findings
first. **D**: significant risk. **F**: don't use it.

Report the computed grade in `overall_grade`, and show the computation in the
"Grade derivation" section. If you believe the computed grade misrepresents
the SDK, say so in the Verdict with the reason, but still report the computed
grade. That disagreement is input for the next prompt version.

## Output

Write the report as one Markdown file, at the path the person running the
audit gives you. The default is
`project/sdk-quality-check/reports/<sdk>-<YYYY-MM-DD>-<model>.md`, or
`<sdk>/audit/reports/<YYYY-MM-DD>-<model>.md` for repos that have graduated to
their own `audit/` directory. Use this exact structure. Another agent will read
the report later to decide whether to trust this SDK without redoing all the
work, so make every claim checkable.

```
---
sdk: <repo name, e.g. sdk-node>
sdk_class: <backend | frontend | wrapper>
upstream: <wrapper only: resolved package@version list; otherwise omit>
commit: <full SHA audited>
version: <latest tag, or `git describe --tags`>
audit_date: <YYYY-MM-DD>
auditor_model: <exact model ID>
auditor_harness: <e.g. Claude Code, Cursor, custom>
prompt_version: 3
integration_test_data_ref: <tag the unit CI pins, or n/a>
chaos_integration_test_data_ref: <tag the chaos workflows pin, or n/a>
chaos: <ran | read-only | n/a>
overall_grade: <A | B+ | B | C | D | F, computed per the rubric>
findings: { critical: N, high: N, medium: N, low: N }
claims: { verified: N, partly: N, not_verified: N, refuted: N, na: N }  # S1-S4 only
---

# <sdk> audit, <date>

## Verdict
Three to six sentences a CTO can read: would you run this in production,
what's the biggest risk, and how confident are you.

## Scorecard
| Area | Grade | One-line reason |
(one row each for: Correctness, Resilience, Concurrency, Security,
Test suite, API design, Maintainability)

## Grade derivation
The rubric inputs (counts, Medium findings in Resilience or Security, S1-S4
verdicts, lowest scorecard area), the first row whose conditions all hold,
and, if one blocks the next row up, the condition that blocks it.

## What I ran
Every command with a short result (pass/fail counts, versions). Say what you
skipped and why, including chaos.

## Findings
One subsection per finding, most severe first:
### [SEVERITY] Short title
- Area: one scorecard area
- Where: file:line (paths relative to the repo root, no links)
- What: the defect
- Why it matters: the production effect
- Evidence: command and output, a code excerpt, or a reproduction
- Reference: (divergence claims only) the ITD case or reference file:line
- Inherited from: (wrappers only, upstream defects) package@version
- Found in: (step 8 findings only) step 8
- Suggested fix: one or two sentences

## Vendor claims
| Claim | Verdict | Evidence |
(S1-S4, then I1-I4, then one `D: <decision>` row per recorded decision
checked; only S1-S4 count in the rubric)

## Chaos
The chaos pin vs the unit pin, every stubbed or silently defaulted expression
(file:line and the scenarios that use it), and what the scenarios would and
would not catch. For classes without a rig: "N/A (no chaos rig)".

## Test suite assessment
Coverage of the shared contract (or of the hand-written suite), tests that
can't fail, and gaps.

## Cross-SDK hazard checklist
| Cluster | Status | Note |
(one row per cluster in META-ANALYSIS section 2; written in step 8)

## What I did not check
Be specific. This is where the next auditor should start.

## Comparison with earlier audits
(or "First audit.")

## Likely cross-SDK issues
Defects or gaps in this SDK that probably exist in sibling SDKs too. For
each one:
- the pattern, and where it is in this SDK (file:line);
- why it is probably shared: a spec gap, an ITD gap, or a pattern copied
  between SDKs;
- which sibling SDKs to check, and anything you already checked there
  (file:line);
- the contract fix that would catch it everywhere: an ITD case, a chaos
  scenario, or a spec line.
Write "None found" if there are none.

## Adaptations to prompt
Everything you did differently from this prompt:
- the commands you derived from CI, and any you substituted;
- how you translated the step 5 areas into this language's terms;
- steps or claims you marked N/A beyond the class table, and why;
- anything you skipped or could not run, and why.
Write "None" only if you followed the prompt exactly.

## How to spot-check this report
Three to five quick commands or file reads a later agent can run to confirm
the most important findings without redoing the audit.
```
