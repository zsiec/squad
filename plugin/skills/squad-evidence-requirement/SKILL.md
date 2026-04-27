---
name: squad-evidence-requirement
description: Every "tests pass" / "build green" / "feature works" claim needs the actual command output pasted into the conversation. Bare assertions are worth zero.
allowed-tools:
  - Bash
  - Read
disable-model-invocation: false
---

# Evidence requirement

A claim is not done because you are confident it is done. A claim is done when the verification artifact for that claim exists in the conversation, in machine-readable form, that anyone reading later could re-run themselves.

## When to use this skill

Invoke this skill any time you are about to write the words "tests pass," "build is clean," "this works," "looks good," or any equivalent. Also invoke before running `squad done` or `/done`. The pre-flight check for `done` is whether the evidence pastes are present.

## The rule

Two artifacts per verification, not one:

1. **Paste** the relevant trailing output line into the conversation. Do not paraphrase. Do not summarize. Do not say "tests pass" — paste the line that proves it.
2. **Attest** the same run via `squad attest`, writing the output to a file the ledger can keep. The paste is for the humans and agents reading the thread now; the attestation is for the next session, the reviewer, and `squad done`'s gate.

Concrete shape (capture once, use both ways):

```bash
go test ./... 2>&1 | tee .squad/attestations/_tmp_test.txt
squad attest <ITEM-ID> --kind test --command "go test ./..." --output .squad/attestations/_tmp_test.txt
```

Then paste the trailing `ok` line into chat. One run, both artifacts.

Concretely:

- **Test runner (Go, Node, Python, Rust, etc.):** include the trailing `ok` / `PASS` / `passed` summary line. If many packages, summarize as `ok: N/M packages, <duration>` plus any FAIL block verbatim.
- **Type checker:** include the trailing `<N> errors, <N> warnings` line.
- **Linter / formatter:** silent success or the actual error output. No paraphrasing.
- **Build:** silent success or the actual error output.
- **Manual verification (browser, CLI smoke test, integration):** explicitly state what you observed in concrete terms. Generic statements like "works as expected" do not count.
- **Skipped a gate?** Say so and why ("no UI changes, skipping vitest"). Silence reads as omission.

## Why this matters

The next session reading your conversation cannot distinguish "I ran the tests and they passed" from "I think they would pass if I ran them" unless you paste the output. Without the paste, your "done" is unverifiable. Unverifiable "done" rots the team's trust in the backlog: future agents start re-running everything to be sure, which doubles wall-clock per item and defeats the point of the close-out.

The paste is also a forcing function on you. If you cannot paste a green line, you have not actually verified — you have rationalized. Catch yourself in the rationalization before it ships.

## How to apply

1. Run the verification command in a tool call, capturing stdout.
2. Read the output. Find the line that summarizes the result.
3. Paste that line, in a code block, into the conversation directly under the command that produced it.
4. If the line is buried in noise, paste the noise too — others may need it for diagnosis.
5. If the command failed, paste the failure block verbatim and stop. Do not move on.

## Anti-patterns to avoid

- "All tests pass" with no paste. Cannot be verified.
- "Should pass" / "would pass." Either it passed or you did not run it.
- Pasting a green line for a different command than the one you claim to have run. Match the paste to the claim.
- Pasting a stale paste from earlier in the session as if it were the latest run. Re-run; paste the latest.
- Skipping the paste because "the commit hook would have caught it." The commit hook runs after you claim done, not before; the order matters.

## What kinds to record

`squad attest --kind <kind>` accepts a few standard kinds. Pick the one that matches the run:

- **`--kind test`** — any unit / integration / e2e command. `go test`, `pytest`, `vitest`, `cargo test`, `npm test`. Mandatory for `bug` / `feature` / `task` items; the per-item `evidence_required: [test]` field gates `squad done` on at least one attestation of this kind.
- **`--kind review`** — a reviewer-agent run. Typically captured by reviewer agents themselves per `squad-reviewing-as-disprove`; the output is the reviewer's findings document or transcript.
- **`--kind manual`** — manual verification. Browser smoke test, CLI demo, "I clicked through and the modal opened." Write down what you actually did and what you observed; the `--output` file is your description.
