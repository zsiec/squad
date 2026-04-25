# squad

Project-management framework for software work done with AI coding agents. Atomic claims, typed chat, file-touch tracking, hygiene daemon, web dashboard, and a Claude Code plugin that encodes the operating loop as enforceable patterns.

> ⚠️ **Status:** under active development, pre-1.0.

## Install

```bash
brew install zsiec/tap/squad        # (planned, Phase 14)
go install github.com/zsiec/squad/cmd/squad@latest
```

## Quickstart

```bash
cd ~/dev/your-project
squad init
squad next
```

### Cross-repo views

Once `squad init` has run in two or more repos, the global DB knows about all of them. From any repo:

```bash
squad workspace status            # per-repo summary table
squad workspace next --limit 10   # top P0/P1 across every repo
squad workspace who               # every agent in every repo, last activity
squad workspace list              # all known repos
squad workspace forget <repo_id>  # remove a repo (e.g. after deleting it locally)
```

Filter scope with `--repo current`, `--repo other`, or `--repo id1,id2,id3`.

See `docs/adopting.md` once it lands (Phase 13).

## Recipes

- [Auto-archive items on PR merge (GitHub Actions)](docs/recipes/github-actions.md)

## License

MIT — see [LICENSE](LICENSE).
