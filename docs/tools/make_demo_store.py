#!/usr/bin/env python3
"""Build a synthetic Claude Code session store for the README screenshots.

The images in the README are real renders, but rendering them from a real store
would publish whatever happens to be in it. This writes a fabricated store with
the same record shapes so the screenshots are reproducible and contain nothing
private.

    python3 docs/tools/make_demo_store.py /tmp/demo-store
"""
import json
import os
import sys
import time
import uuid

HOME = "/Users/you"
REPO = f"{HOME}/code/acme-api"

# (cwd, branch, title, model, ai_titled, age_seconds, exchanges)
SESSIONS = [
    (REPO, "main", "Fix flaky auth middleware test", "claude-opus-5", True, 900, [
        ("you", "the auth middleware test fails about one run in five"),
        ("claude", "It's a shared clock: the token TTL is computed once at package init, so a "
                   "test that runs more than a second later sees it expired. Injecting a clock "
                   "into the middleware fixes it."),
        ("you", "do that, and add a test that fails without the fix"),
        ("claude", "Done — `auth.Middleware` now takes a `Clock`, the test advances it "
                   "explicitly, and reverting the change makes the new test fail."),
    ]),
    (REPO, "main", "Add pagination to the /orders endpoint", "claude-sonnet-5", True, 7200, [
        ("you", "add cursor pagination to /orders"),
        ("claude", "Added an opaque cursor over `(created_at, id)` so pages stay stable when "
                   "rows are inserted mid-scan, with the limit capped at 200."),
    ]),
    (REPO, "main", "Why is the Docker image 1.2GB?", "claude-sonnet-5", False, 86400 * 2, [
        ("you", "why is our docker image 1.2gb"),
        ("claude", "The build stage is the final stage, so the Go toolchain and module cache "
                   "ship with it. A two-stage build on distroless takes it to 31MB."),
    ]),
    (f"{REPO}/web", "main", "Dark mode flicker on first paint", "claude-opus-5", True, 5400, []),
    (f"{REPO}/web", "feat/charts", "Chart legend overlaps on narrow screens", "claude-sonnet-5", True, 86400 * 3, []),
    (f"{REPO}/worker", "main", "Retry backoff for the webhook queue", "claude-opus-5", True, 86400 * 5, []),
    (f"{HOME}/code/infra", "main", "Terraform plan wants to replace the RDS instance", "claude-opus-5", True, 3600, []),
    (f"{HOME}/code/infra", "main", "scheduled: nightly-cost-report", "claude-haiku-4-5", False, 43200, []),
    (f"{HOME}/code/dotfiles", "main", "Make zsh startup faster", "claude-haiku-4-5", True, 86400 * 4, []),
    (f"{HOME}/code/blog", "main", "Draft a post about cursor pagination", "claude-opus-5", True, 86400 * 6, []),
    (f"{HOME}/code/scratch", "HEAD", "Compare JSON parsers for a 40MB file", "claude-sonnet-5", True, 86400 * 9, []),
    (f"{HOME}/code/acme-cli", "main", "Ship completions for zsh and fish", "claude-sonnet-5", True, 86400 * 12, []),
    (f"{HOME}/code/acme-cli", "main", "Reduce heap allocations in the parser", "claude-opus-5", True, 86400 * 16, []),
    (f"{HOME}/code/notes", "main", "Summarise the on-call handover", "claude-haiku-4-5", False, 86400 * 21, []),
    (f"{HOME}/code/acme-api-tools", "main", "Backfill script for the orders table", "claude-sonnet-5", True, 86400 * 30, []),
]


def munge(path):
    """Claude Code's project-directory name for a working directory."""
    return path.replace("/", "-")


def write_session(store, cwd, branch, title, model, ai_titled, age, exchanges):
    project = os.path.join(store, munge(cwd))
    os.makedirs(project, exist_ok=True)
    sid = str(uuid.uuid4())
    path = os.path.join(project, sid + ".jsonl")

    common = {"cwd": cwd, "gitBranch": branch, "sessionId": sid, "version": "2.1.227"}
    turns = exchanges or [("you", title), ("claude", "Working on it.")]
    lines = []
    for who, text in turns:
        if who == "you":
            lines.append({"type": "user", "origin": {"kind": "human"},
                          "message": {"role": "user", "content": text}, **common})
        else:
            lines.append({"type": "assistant",
                          "message": {"role": "assistant", "model": model,
                                      "content": [{"type": "text", "text": text}]}, **common})
    if ai_titled:
        lines.append({"type": "ai-title", "aiTitle": title, "sessionId": sid})

    # Compact separators matter: Claude Code writes compact JSON, and the
    # scanner's byte prefilter looks for `"type":"user"` with no space.
    with open(path, "w", encoding="utf-8") as f:
        for line in lines:
            f.write(json.dumps(line, separators=(",", ":")) + "\n")

    stamp = time.time() - age
    os.utime(path, (stamp, stamp))


def main():
    store = sys.argv[1] if len(sys.argv) > 1 else "/tmp/cl-demo-store"
    os.makedirs(store, exist_ok=True)
    for session in SESSIONS:
        write_session(store, *session)
    # A settings file so the model footer shows a configured default first.
    with open(os.path.join(os.path.dirname(store), "settings.json"), "w") as f:
        json.dump({"model": "opus"}, f)
    print(f"wrote {len(SESSIONS)} synthetic sessions to {store}")
    print(f"HOME for rendering: {HOME}    repo: {REPO}")


if __name__ == "__main__":
    main()
