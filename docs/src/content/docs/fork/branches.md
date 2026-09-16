---
title: Branches and upstream PRs
---

Every change in this fork is open upstream. If they all land, the fork stops being
necessary and the tablet can point at a stock build again. Status below is as of
2026-09-16.

## Branches

| Branch | Base | What it holds |
|--------|------|---------------|
| `v6-upstream-sync` | `joagonca/master` | the v6 export work, 24 commits: per-page format routing, schema 2 page lists, page templates, ink placement and the landscape turn |
| `fix-page-pairing` | `ddvk/master` | one commit, `internal/storage/models/archive.go`, so annotations land on the page they were written on |
| `fix-landscape-rotation` | `ddvk/master` | one commit, turning the ink a quarter turn on a landscape page |
| `local-build` | `v6-upstream-sync` | adds one `go.mod` replace so the build uses `giovi321/rmc-go`. This is the branch that gets built and deployed |
| `points-probe` | `v6-upstream-sync` | a test-only helper that dumps a page's stroke points, used to measure ink placement |
| `measurement-tools` | `v6-upstream-sync` | the scripts under `tools/measure/` that the measurements came from |
| `master` | `ddvk/master` | upstream, plus this documentation |

## Open upstream

| PR | Repo | Branch | What it fixes |
|----|------|--------|---------------|
| [#484](https://github.com/ddvk/rmfakecloud/pull/484) | `ddvk/rmfakecloud` | `fix-page-pairing` | annotations landing on the wrong page, reported in #228 |
| [#483](https://github.com/ddvk/rmfakecloud/pull/483) | `ddvk/rmfakecloud` | `fix-landscape-rotation` | sideways annotations on landscape documents, reported in #286 |
| [#4](https://github.com/joagonca/rmfakecloud/pull/4) | `joagonca/rmfakecloud` | `v6-upstream-sync` | the v6 export work, which feeds `ddvk/rmfakecloud#441` |
| [#13](https://github.com/joagonca/rmc-go/pull/13) | `joagonca/rmc-go` | `fix-trailing-blank-page` | the last page of a notebook going missing |
| [#12](https://github.com/joagonca/rmc-go/pull/12) | `joagonca/rmc-go` | `add-license` | the LICENSE file the README promises |

`#483` and `#484` are each one commit against current `ddvk/master` and are independent
of everything else, so either can land on its own.

## Why the dependency swap is not on v6-upstream-sync

`v6-upstream-sync` is the head of `joagonca/rmfakecloud#4`. A pull request that
redirects a dependency to its author's own fork is not something a maintainer can
merge, so the `replace` lives on `local-build` and the PR branch stays clean. Anything
built for deployment comes from `local-build`; anything offered upstream comes from the
branch that carries only the fix.
