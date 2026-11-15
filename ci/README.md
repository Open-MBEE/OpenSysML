# CI throttle and watch tools

These tools share the repository's GitHub Actions runners between many pull requests and the merge
queue. The rule they enforce is that **the merge queue never waits for runners**. Ordinary
pull-request runs are capped, and none is admitted while a `merge_group` run has queued or waiting
jobs. They are operator tools: they run on a maintainer's or an agent's machine with an
authenticated `gh`, never in CI. Nothing in the Go build or test suite imports them.

| Tool | What it does |
| --- | --- |
| `watcher.py` | The throttle and watch loop. It caps concurrent ordinary pull-request runs, queues the over-cap ones in priority order, re-starts them when a slot frees, holds every slot for the merge queue, and reports each decision. |
| `starve_sweep.py` | Frees runners for the merge queue. While a merge-queue run has waiting jobs, it cancels ordinary pull-request runs that have jobs but none started. When a merge-queue run older than `starve_grace_seconds` has started no job at all, it also cancels every ordinary run with a job in progress except the one closest to finishing. Each run it cancels is queued for the watcher to re-run (`auth_rerun.json`). |
| `infra_rerun.py` | Classifies the failed jobs of the runs in `infra_runs.json` as infrastructure (setup steps, DNS, apt mirrors) or real, and re-runs (`--failed`) only the infrastructure failures once the infrastructure has recovered. |
| `drycheck.sh` | Dry-merges (`git merge-tree`) each pull request in `ready.txt` onto its base branch's head and prints CLEAN or DIRTY. |
| `drywatch.sh` | Runs `drycheck.sh` every time the base branch moves and appends the results to `dry.log`. |
| `poll_develop.sh` | Waits until the base branch moves off a given SHA, then prints the new head. |
| `throttle_common.py` | Shared code: configuration, control files, and the guarded `gh` wrapper. `throttle_common.py get KEY` prints one configuration value for the shell tools. |

## Safety guarantees

The code keeps these, and `tests/test_ci_tools.py` checks them:

- **Fail closed.** A `gh` call that the decision depends on raises or returns nothing, and then
  nothing is cancelled or re-run on that pass. An unreadable merge queue counts as a busy one.
- **Protected runs are never cancelled.** This covers `merge_group` runs (`gh-readonly-queue/*`),
  tags (`v<digit>…`), and the branches in `exempt_branches` and `exempt_prefixes` (the defaults
  are `develop`, `main`, `release/` and `hotfix/`). `watcher.py` checks this again inside its
  `cancel`, from the run's own event and branch.
- **No cancel without authorization.** The watcher cancels a run only if its id is in
  `cancel_auth.json`. A run that has started a job is never cancelled, with one exception: a run
  that has only finished its preflight job (`preflight_jobs`) may be cancelled under the review
  gate. The sweeper's cancels are authorized by `starve_cancel.json`, and the sweeper stops when
  that file is removed.
- **Freeze.** While `freeze.json` exists, the `gh` wrapper skips every mutating call (`run
  cancel|rerun`, `pr close|reopen|edit|ready|merge`, `workflow run`, `api -X …`) and logs it, and
  the watcher's cap drops to 0. Use it during a GitHub Actions outage.
- **Dry run.** With `--dry-run`, every tool reads GitHub as usual but logs each mutation instead of
  making it. `--once` runs a single pass and exits.

## Setup

Requirements: Python 3.10+, `git`, `curl`, and a `gh` that is logged in with permission to cancel
and re-run workflow runs.

```bash
mkdir -p ~/ci-throttle
cp ci/config.example.json ~/ci-throttle/config.json      # then edit it
cp ci/tiers.example.json ~/ci-throttle/tiers.json        # optional: the priority order
python3 ci/watcher.py --dry-run --once                   # rehearse one pass, then read ~/ci-throttle/watch.log
```

Each tool finds its configuration in this order: `--config PATH`, then `$CI_THROTTLE_CONFIG`, then
`~/ci-throttle/config.json`. The state directory holds the control files and logs. It defaults to
the directory that contains the configuration file.

| Key | Meaning |
| --- | --- |
| `repo` | `OWNER/NAME` (required). |
| `state_dir` | Directory for control files and logs (default: the configuration file's directory). |
| `checkout` | A local clone, used by `drycheck.sh` for dry merges. |
| `base_branch` | The integration branch the merge queue targets (default `develop`). |
| `workflows` | Workflow names the cap counts. The first is the pull-request workflow whose runs are re-started (default `["PR"]`). |
| `preflight_jobs` | Cheap jobs that do not count as "started" (default `["Changed areas"]`). |
| `required_check` | The aggregate required check (default `Build and test`). |
| `cap` | Concurrent ordinary runs (default 3). It drops to 1 while a merge-queue run is active, and to 0 under a freeze or a runner-priority hold. |
| `interval_seconds`, `fast_poll_seconds` | Time between full passes, and between the fast polls for new runs inside that time (defaults 120 and 10). |
| `exempt_branches`, `exempt_prefixes` | Branches whose runs are never counted or cancelled. Tags and `gh-readonly-queue/` are always exempt. |
| `exempt_body_tag` | A line in a PR body that exempts the PR: its runs are admitted automatically and never cancelled. With `tiers.json`, the exemption applies only to tier-1 PRs. |
| `runner_pin` | `{"file", "require", "forbid"}`: before a close/reopen, the workflow file at the PR's merge ref must contain `require` and must not contain `forbid`. Leave it out to skip the check. |
| `ci_priority` | `{"after_pr", "title_re", "path_re"}`: a new PR into the base branch whose number is above `after_pr`, and whose title or changed paths match, is admitted as a CI priority PR. With `tiers.json`, only PRs in tiers 1 and 2 qualify. `null` turns this off. |
| `known_flakes` | `[{"job", "test", "note"}]`: a red result whose only failures are this job's (plus the required check) and only this test is reported as the known flake, not as the PR's failure. |
| `job_timeout_minutes` | A cancelled job that ran at least this long is reported as a timeout (default 59). |
| `starve_grace_seconds` | How old a merge-queue run with no started job must be before the sweeper counts it as starved (default 180). |

## Starting and stopping

Start each loop from a shell that has `gh` credentials. A subprocess without them fails every `gh`
call while it keeps running. Record the PID:

```bash
nohup python3 ci/watcher.py > ~/ci-throttle/watcher.out 2>&1 & echo $! > ~/ci-throttle/watcher.pid
echo '{"by": "maintainer", "rule": "the merge queue never waits for runners"}' > ~/ci-throttle/starve_cancel.json && nohup python3 ci/starve_sweep.py > ~/ci-throttle/sweep.out 2>&1 &
nohup python3 ci/infra_rerun.py > ~/ci-throttle/infra.stdout 2>&1 &   # exits once every run in infra_runs.json has settled
nohup ci/drywatch.sh > /dev/null 2>&1 &
```

- **Stopping the watcher.** Create `stop.json` in the state directory, or set `"stop": true` in
  `state.json`. It exits at the start of its next pass. Remove the file before you start it again.
- **Stopping the sweeper.** Remove `starve_cancel.json`.
- **Stopping the shell loops.** Kill them by PID.
- **Restarting the watcher.** Check first with `pgrep -af watcher.py` that no copy is running,
  because `watcher.pid` can be stale. Read the PID in one command and kill it in another: a
  `pkill -f watcher.py` run from a shell whose own command line contains the pattern kills that
  shell too.
- **Reading the output.** The watcher writes `watch.log` (every decision) and `outbox.jsonl` (the
  `REPORT` lines, one JSON object per line, for whoever relays them). The sweeper also writes to
  `watch.log`. `infra_rerun.py` writes `infra.out`, and `drywatch.sh` writes `dry.log`.
- **Transient `gh` failures.** A burst of `gh auth login` failures in one pass is usually a token
  refresh, and the next pass recovers. If the failures continue for many passes, restart the
  watcher.

## Priority order

`tiers.json` (`{"tiers": [[pr, …], [pr, …], …]}`) is the binding order. Tier 1's PRs come first,
in list order, then tier 2's, and so on. Every PR not listed follows by PR number, so the oldest
goes first. Without the file, every PR goes by PR number. This order is used for:

- the queue of runs waiting for a slot;
- the authorized re-runs (`auth_rerun.json`), which are taken in tier order and only when the
  re-run's rank is no worse than the best runnable queue entry's;
- the order in which the sweeper queues its re-runs.

Merge-queue runs come before every tier. The PR numbers in `tiers.json` are the current
coordination state, so `tiers.example.json` only shows the file's shape.

## Control files

All control files live in the state directory. Each one is optional, and the watcher re-reads
them on every pass, so you edit them while it runs. Run ids are numbers, and SHAs may be prefixes
unless stated otherwise.

| File | Shape | Effect |
| --- | --- | --- |
| `freeze.json` | anything | Skip every mutation (all tools). Cap 0. |
| `stop.json` | anything | The watcher exits. |
| `tiers.json` | `{"tiers": [[pr]]}` | Priority order (see above). |
| `cancel_auth.json` | `[run]` | Runs the watcher may cancel. |
| `auth_rerun.json` | `[{"pr", "run", "head", "full"}]` | Authorized re-runs, taken when a slot frees and no merge-queue run waits. Each one is `gh run rerun --failed`, or a plain `gh run rerun` of every job when the entry has `"full": true`. An entry is skipped if the PR's head has moved off `head`. The sweeper appends to this file. |
| `starve_cancel.json` | `{"by", "rule"}` | Authorizes and runs `starve_sweep.py`. |
| `release.json` | `[{"pr", "sha"}]` | Heads released to CI under the review gate. `"pr": "*"` with a full 40-character SHA releases a head before its PR exists. |
| `review_gate.json` | `{"since"}` | Admit only released heads that have no unresolved review threads. Unreleased runs caught with only the preflight done are cancelled (authorization still applies) and queued. |
| `exempt.json` | `[pr]` | PRs with the exemption (see `exempt_body_tag`). |
| `ci_prio.json`, `ci_flagged.json` | `[pr]` | CI priority PRs admitted, and those flagged instead because they change files of an admitted one. The watcher writes both, and you may edit them. |
| `nocancel.json` | `[pr]` | Never cancelled before start. Their runs queue without being cancelled and take the first free slot. |
| `pinned.json` | `[pr or branch]` | Always hold a slot. If their run is cancelled, it is re-run at once. |
| `ignore.json` | `[pr]` | Runs not counted against the cap and never touched. |
| `backmerge.json` | `[branch]` | Release back-merge branches. Never cancelled for being dirty or for lacking the strict base. |
| `hold.json` | `{"pr": [sha]}` | The owner has announced a push. The PR is skipped while its head is one of these SHAs. |
| `blocked.json` | `{"prs": [pr], "until": pr, "why"}` | Cancel these PRs' not-started runs and keep them out of the queue. The block lifts once `until`'s run is executing. |
| `gate.json` | `{"pr", "allow": [pr]}` | Until `pr` merges, only `allow` may run. Every other not-started run is cancelled and queued. |
| `defer.json` | `{"pr": [blocker pr]}` | Hold `pr` while any blocker is open. |
| `reserve.json` | `{"pr"}` | Keep one slot free for `pr`'s next run. |
| `solo.json` / `solo_arm.json` | `{"pr", "run"}` / `{"pr", "head"}` | Sole-priority hold: only `pr` runs until its run finishes. `solo_arm.json` arms the hold for the PR's next push away from `head`. |
| `priority_hold_<pr>.json` | `{"pr", "run", "head", "after", "label"}` | Runner-priority hold: no other new run starts until `run` finishes. With `"run": null`, the hold binds to the first run on a head that is not `after`. `infra_rerun.py` and the stale-check re-run write these files. |
| `wide_cap.json` | `{"prs", "cap", "seen": {}, "done": []}` | Raise the cap to `cap` until each listed PR's next run has finished. |
| `release_hold.json` | `{"reason", "exempt": [pr]}` | No ordinary slot is granted. A new run that starts anyway is cancelled (authorization still applies) and requeued. |
| `outage.json` | `{"probes": [url]}` | No slot is granted until every probe URL answers a ranged GET with 200 or 206 twice, at least a minute apart. The file is then removed. |
| `strict_base.json` | `{"sha"}` | Every head must contain this commit. Heads without it are parked as needing a base merge. |
| `need_base.json` | `"sha"` | A green result on a head that lacks this commit is reported for the maintainer to decide. |
| `reopen_ok.json` | `[pr]` | PRs whose cancelled run may be restarted by one close/reopen. This happens only when `refs/pull/N/merge` sits on the current base head (or the head is in `stale_ok.json`) and carries the runner pin. Each entry is used once. |
| `stale_ok.json` | `{"pr": sha}` | Exact heads checked CLEAN with no file overlap against the newer base, which may be reopened on an older merge ref. |
| `restart_ok.json` | `[run]` | Non-cancelled runs that may still be restarted. |
| `ready_ok.json` | `[pr]` | Drafts the watcher may un-draft (`gh pr ready`) when they reach a slot. Every other draft is left alone. |
| `draft_excl.json` | `[pr]` | Drafts kept out of the un-draft queue even if they are listed in `ready_ok.json`. |
| `ext_ok.json` | `[run]` | Runs started on the maintainer's order. Never cancelled, and not counted as the watcher's own cancels. |
| `unreleased.json` | `[run]` | Runs re-run without a release. Their result does not qualify the head. |
| `fail_ok.json` | `{"run": {"jobs": [name], "agg_only": [name], "label", "why"}}` | An authorized exception: a red result that failed only on `jobs` is reported as validated. |
| `cutoff.json` | `{"before": iso time}` | Queued re-runs of runs created before this time are not started. |
| `infra_runs.json` | `[{"run", "pr", "head", "hold"}]` | The runs `infra_rerun.py` tracks, in re-run order. `head` is the full SHA. With `hold`, the run blocks the runs after it until its first re-run. |
| `ready.txt` | one PR number per line | The PRs `drycheck.sh` dry-merges. |

The tools write these files themselves: `state.json` (holders, queue, parked PRs, the watcher's own
cancels, merge-queue entries), `watch.log`, `outbox.jsonl`, `infra.out` and `dry.log`.

## Standing rules

These rules come from running the tools on this repository and still apply.

**Merge queue**

- The merge queue never waits for runners. No ordinary run is released while a `merge_group` run
  is queued or has waiting jobs. Between two queue builds, a non-empty GraphQL
  `mergeQueue(branch:…).entries` also counts as a busy queue: the next build appears about a
  minute after the previous one finishes, and an ordinary run admitted in that gap delays it.
- The sweeper's two cases follow the maintainer's rule "cancel all normal PR runs, except the
  closest to finishing; keep the merge PR run":
  - **Waiting.** While a merge-queue run has queued or waiting jobs, every ordinary run whose
    heavy jobs are all still queued is cancelled.
  - **Starved.** A merge-queue run older than `starve_grace_seconds` that has started no job means
    ordinary runs took the runners first, typically in the gap between one queue build finishing
    and the next being created. Every ordinary run with a job in progress is cancelled except the
    one with the fewest unfinished jobs. Ties go by tier rank.

  Each successful cancel is queued in `auth_rerun.json`, one entry per PR, in tier order. A
  failed cancel is logged and not queued.
- Never cancel or hold a `merge_group` run, a tag's runs, or a `release/`/`hotfix/` run.
  Re-running a cancelled release run is the maintainer's call.
- When a merge-queue entry fails and stays UNMERGEABLE, the entries behind it keep building on its
  failed group commit. Remove the failed entry so that the rest rebuild on the base branch.
- Merging and enqueueing are the maintainer's decisions. An entry enqueued through GraphQL
  `enqueuePullRequest` is attributed to the identity that enqueued it.

**Which runs count, and what cancelling means**

- A run is "started" only according to its heavy jobs (`in_progress` or `completed`). Never use
  the top-level `status`, which stays `queued` while any job waits, and never use the preflight
  job, which always completes first.
- A queued run cannot be deprioritized. Holding it means cancelling it, which needs
  authorization. `timeout-minutes` counts from the moment a job starts on a runner, not from
  queue time.
- GitHub can leave a run with zero jobs `queued` for days. Both cancel and force-cancel refuse
  it, and a cancelled one cannot be retried. It uses no runner, so ignore it. The watcher doesn't
  count such runs, and the sweeper never cancels them.

**Re-triggering CI**

- Re-triggers are the maintainer's call. Do not push empty commits. Do not run `gh run rerun`
  outside `auth_rerun.json` or `infra_runs.json`. Do not push a base-branch merge whose only
  purpose is to restart CI. A push freeze stops every push.
- A `pull_request` run tests `refs/pull/N/merge`, which GitHub may leave on an old base. A
  `gh run rerun` replays that old merge commit, and a no-op base `PATCH` does not refresh it.
  Before you call a run validated, read the job log's `Merge <head> into <base>` line, and its
  `Image:` line when a failure appears on some runners only. A fresh push, or a close/reopen once
  the merge ref is on the current base, rebuilds the ref.
- The pull-request workflow's concurrency group `pr-<N>` cancels one of two runs on the same
  head. Check the run's annotations before you blame the watcher. Do the same for a cancelled job
  that reports "exceeded the maximum execution time".

**Ordering and re-merges**

- Skip a PR whose `mergeable_state` is dirty until its head changes. Cancel its not-started runs.
- A held PR merges the base branch (`--no-ff`, never a rebase) only when the base branch's new
  commits overlap its files or the baselines it regenerates, or when its dry merge stops being
  clean. Otherwise its head stays releasable, because the merge queue tests the real merge.
  Stacked PRs pick up a base-branch fix bottom-up: each child merges its own base.
- A clean dry merge onto the base branch is not enough for a multi-entry queue. Two PRs that both
  re-record the same generated baseline merge cleanly one at a time but conflict with each other.
  Two PRs that both touch one Go package can merge textually and still fail to compile. Before
  you post a ready list, run the dry merges in sequence, in queue order, and run
  `go build ./... && go vet` over the shared packages.
- Pull-request pushes run the quick tier, the merge queue runs the queue tier, and the full tier
  runs on the `ci:full` label, on an unclaimed path, and nightly (CONTRIBUTING.md, "GitHub
  Actions"). Never add `ci:full`. Un-drafting a PR does not start a full-tier run.
- Throttle only while runners are actually starved. Heavy gating (one slot, pushes sequenced by
  the watcher) costs more throughput than it saves.

**Operating**

- A setup-step failure (installing Z3 or Octave, provisioning a corpus, `apt-get update failed` on
  the Ubuntu mirrors) is an infrastructure outage, not the PR's fault. Track the run in
  `infra_runs.json` and do not re-run it by hand.
- A maintainer's bare "stop" means stop your own actions. Do not stand down the watcher or lift
  holds unless the watcher is named.
- An un-draft or a re-run by a shared bot identity does not by itself implicate the watcher. Read
  the watcher's own log (`gh pr ready` lines, the `ready_ok.json` list) before you blame it.
