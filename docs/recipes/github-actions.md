# Recipe: auto-archive squad items on PR merge

This recipe wires squad to GitHub so that merging a PR automatically marks the
linked item `done` and moves its file to `.squad/done/`. No backlog IDs leak
into branch names or commit messages — the link travels in a hidden HTML
comment in the PR description.

## What you get

- One CI workflow file (~30 lines).
- Agents do not need to remember to run `squad done` after merging.
- Branch names, commit messages, and the merge commit stay free of PM IDs.
- Idempotent: re-running the workflow on an already-archived item is a no-op.

## How the link works

A single hidden comment in the PR body:

    <!-- squad-item: BUG-001 -->

The comment is invisible in the rendered PR view (GitHub strips HTML comments
from Markdown output), but `gh pr view --json body -q .body` returns the raw
text. `squad pr-close` extracts the first match with a regex and acts on it.

## Agent flow (end-to-end)

```bash
# 1. Pick work
squad next                          # see what is ready
squad claim BUG-001                 # take it

# 2. Do the work, commit changes (no BUG-001 in messages or branch name)
git checkout -b fix-the-thing
# ... edit, test, commit ...

# 3. Open the PR with the marker embedded
squad pr-link BUG-001 | head -1 > /tmp/marker.txt   # first line is the HTML marker; trailing line is human-readable
gh pr create \
  --title "fix: stop the thing from doing the bad" \
  --body  "$(cat <<EOF
## Summary

Replaced the broken handler with a less-broken handler.

## Test plan

- [x] go test -race ./...

$(cat /tmp/marker.txt)
EOF
)"

# 4. Get review, merge.
# 5. The action runs squad pr-close, item moves to .squad/done/.
```

If you prefer, `squad pr-link --write-to-clipboard BUG-001` puts the marker on
your system clipboard, and `squad pr-link --pr 42 BUG-001` appends it to an
already-open PR via `gh pr edit`.

## Install

In your repo:

```bash
mkdir -p .github/workflows
curl -fsSL \
  https://raw.githubusercontent.com/zsiec/squad/main/templates/github-actions/on-pr-merge.yml \
  -o .github/workflows/squad-archive.yml
git add .github/workflows/squad-archive.yml
git commit -m "ci: archive squad items on PR merge"
git push
```

The workflow uses the default `GITHUB_TOKEN`; no secrets to configure.

## Trade-offs

- Workflow needs `permissions: contents: write` to push the archive commit. If branch protection forbids bot pushes, exempt `squad-bot` or use the manual fallback.
- The `concurrency` block serializes runs so back-to-back merges never race on `.squad/items/`.
- The action downloads the latest squad release at runtime. To pin, replace `latest` with a specific tag.
- Only the **first** `<!-- squad-item: -->` marker in a PR body is acted on; archive additional items manually with `squad done <ID>`.

## Manual fallback

If you do not want CI doing this, just run `squad done <ID>` locally after the merge. `pr-link` and `pr-close` are independent of `squad done` — use either, both, or neither.

## Troubleshooting

- **Workflow runs but does nothing.** PR body had no marker. Edit the PR description and re-run.
- **`gh: command not found`.** `gh` is preinstalled on `ubuntu-latest`. On self-hosted runners, ensure `gh` is on PATH.
- **Push rejected by branch protection.** Exempt `squad-bot` or use the manual fallback.
