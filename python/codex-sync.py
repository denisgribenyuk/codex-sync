#!/usr/bin/env python3

import argparse
import hashlib
import json
import os
import shutil
import sqlite3
import subprocess
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path


def git_root() -> Path:
    try:
        result = subprocess.check_output(
            ["git", "rev-parse", "--show-toplevel"],
            encoding="utf-8",
            errors="strict",
            stderr=subprocess.DEVNULL,
        ).strip()
    except subprocess.CalledProcessError as exc:
        raise RuntimeError(
            "Current directory is not inside a Git repository"
        ) from exc

    return Path(result).resolve()


def codex_home() -> Path:
    value = os.environ.get("CODEX_HOME")
    if value:
        return Path(value).expanduser().resolve()

    return Path.home() / ".codex"


def sessions_root() -> Path:
    return codex_home() / "sessions"


def sync_root() -> Path:
    return git_root() / ".codex-sync"


def state_db() -> Path:
    return codex_home() / "state_5.sqlite"


def normalize_path(value: str) -> str:
    value = str(value).strip()

    if value.startswith("\\\\?\\"):
        value = value[4:]

    value = value.replace("\\", "/")
    value = value.rstrip("/")

    if os.name == "nt":
        value = value.lower()

    return value


def belongs_to_repo(session: Path, repo: Path) -> bool:
    repo_normalized = normalize_path(str(repo))

    try:
        with session.open(
            "r",
            encoding="utf-8",
            errors="replace",
        ) as f:
            for line in f:
                if '"cwd"' not in line:
                    continue

                try:
                    item = json.loads(line)
                except json.JSONDecodeError:
                    continue

                payload = item.get("payload")
                if not isinstance(payload, dict):
                    continue

                cwd = payload.get("cwd")
                if not cwd:
                    continue

                if normalize_path(cwd) == repo_normalized:
                    return True

    except OSError:
        pass

    return False


def session_id(session: Path) -> str:
    with session.open(
        "r",
        encoding="utf-8",
        errors="replace",
    ) as f:
        for line in f:
            try:
                item = json.loads(line)
            except json.JSONDecodeError:
                continue

            if item.get("type") == "session_meta":
                payload = item.get("payload", {})
                value = payload.get("id")

                if value:
                    return value

    raise RuntimeError(
        f"Cannot determine session id: {session}"
    )


def sha256(path: Path) -> str:
    h = hashlib.sha256()

    with path.open("rb") as f:
        for chunk in iter(
            lambda: f.read(1024 * 1024),
            b"",
        ):
            h.update(chunk)

    return h.hexdigest()


def find_project_sessions(repo: Path) -> list[Path]:
    root = sessions_root()

    if not root.exists():
        raise RuntimeError(
            f"Codex sessions directory doesn't exist: {root}"
        )

    matches = []

    for path in root.rglob("*.jsonl"):
        if belongs_to_repo(path, repo):
            matches.append(path)

    if not matches:
        raise RuntimeError(
            f"No Codex sessions found for repository:\n{repo}"
        )

    return sorted(
        matches,
        key=lambda p: p.stat().st_mtime_ns,
        reverse=True,
    )


def export_session():
    repo = git_root()
    session_root = sessions_root()

    project_sessions = find_project_sessions(repo)

    latest = project_sessions[0]
    sid = session_id(latest)

    same_thread = []

    for path in project_sessions:
        try:
            if session_id(path) == sid:
                same_thread.append(path)
        except Exception:
            pass

    if not same_thread:
        raise RuntimeError(
            f"No rollout files found for session {sid}"
        )

    target_root = sync_root()
    target_sessions = target_root / "sessions"

    if target_sessions.exists():
        shutil.rmtree(target_sessions)

    exported = []

    for source in same_thread:
        relative = source.relative_to(session_root)
        target = target_sessions / relative

        target.parent.mkdir(
            parents=True,
            exist_ok=True,
        )

        shutil.copy2(source, target)

        exported.append(
            {
                "relative_path": relative.as_posix(),
                "sha256": sha256(target),
            }
        )

    manifest = {
        "session_id": sid,
        "source_repo": str(repo),
        "exported_at": (
            datetime.now(timezone.utc).isoformat()
        ),
        "files": exported,
    }

    target_root.mkdir(
        parents=True,
        exist_ok=True,
    )

    with (
        target_root / "manifest.json"
    ).open(
        "w",
        encoding="utf-8",
    ) as f:
        json.dump(
            manifest,
            f,
            indent=2,
            ensure_ascii=False,
        )
        f.write("\n")

    print("Codex session exported")
    print(f"Session: {sid}")
    print(f"Files:   {len(exported)}")

    for item in exported:
        print(f"  {item['relative_path']}")


def read_manifest():
    manifest_file = sync_root() / "manifest.json"

    if not manifest_file.exists():
        raise RuntimeError(
            "No .codex-sync/manifest.json found. "
            "Export the session on the other computer first."
        )

    with manifest_file.open(
        "r",
        encoding="utf-8",
    ) as f:
        return json.load(f)


def import_session():
    manifest = read_manifest()

    sid = manifest["session_id"]
    files = manifest.get("files", [])

    if not files:
        raise RuntimeError(
            "Manifest contains no session files"
        )

    imported = 0
    imported_paths = []

    for item in files:
        relative = Path(
            item["relative_path"]
        )

        source = (
            sync_root()
            / "sessions"
            / relative
        )

        if not source.exists():
            raise RuntimeError(
                f"Session file missing: {source}"
            )

        destination = (
            sessions_root()
            / relative
        )

        destination.parent.mkdir(
            parents=True,
            exist_ok=True,
        )

        if destination.exists():
            if sha256(destination) != sha256(source):
                timestamp = (
                    datetime.now()
                    .strftime("%Y%m%d-%H%M%S")
                )

                backup = (
                    destination.with_suffix(
                        destination.suffix
                        + f".bak-{timestamp}"
                    )
                )

                shutil.copy2(
                    destination,
                    backup,
                )

                print(
                    f"Backup: {backup}"
                )

        shutil.copy2(
            source,
            destination,
        )

        imported += 1
        imported_paths.append(destination)

    print("Codex session imported")
    print(f"Session: {sid}")
    print(f"Files:   {imported}")

    return sid, imported_paths


def get_thread_metadata(
    conn: sqlite3.Connection,
    sid: str,
):
    row = conn.execute(
        """
        SELECT
            title,
            preview,
            first_user_message,
            source,
            model_provider,
            sandbox_policy,
            approval_mode,
            tokens_used,
            has_user_event,
            cli_version,
            model,
            reasoning_effort,
            history_mode,
            name
        FROM threads
        WHERE id = ?
        """,
        (sid,),
    ).fetchone()

    return row


def ensure_project(
    conn: sqlite3.Connection,
    repo: Path,
) -> str:
    project_name = repo.name

    rows = conn.execute(
        """
        SELECT id, name
        FROM projects
        ORDER BY position ASC
        """
    ).fetchall()

    for project_id, name in rows:
        if name == project_name:
            return project_id

    project_id = str(uuid.uuid4())
    now_ms = int(
        datetime.now(timezone.utc).timestamp()
        * 1000
    )

    max_position = conn.execute(
        """
        SELECT COALESCE(MAX(position), -1)
        FROM projects
        """
    ).fetchone()[0]

    conn.execute(
        """
        INSERT INTO projects (
            id,
            name,
            metadata,
            position,
            created_at_ms,
            updated_at_ms
        )
        VALUES (?, ?, '{}', ?, ?, ?)
        """,
        (
            project_id,
            project_name,
            max_position + 1,
            now_ms,
            now_ms,
        ),
    )

    return project_id


def find_primary_rollout(
    imported_paths: list[Path],
) -> Path:
    if not imported_paths:
        raise RuntimeError(
            "No imported rollout files"
        )

    return max(
        imported_paths,
        key=lambda p: p.stat().st_mtime_ns,
    )


def register_thread_in_desktop(
    sid: str,
    imported_paths: list[Path],
):
    db = state_db()

    if not db.exists():
        print(
            f"Codex state DB not found: {db}"
        )
        print(
            "Skipping Desktop UI registration"
        )
        return

    repo = git_root()
    rollout_path = find_primary_rollout(
        imported_paths
    )

    now = int(
        datetime.now(timezone.utc).timestamp()
    )
    now_ms = now * 1000

    conn = sqlite3.connect(db)

    try:
        project_id = ensure_project(
            conn,
            repo,
        )

        existing = conn.execute(
            """
            SELECT id
            FROM threads
            WHERE id = ?
            """,
            (sid,),
        ).fetchone()

        if existing:
            conn.execute(
                """
                UPDATE threads
                SET
                    rollout_path = ?,
                    cwd = ?,
                    project_id = ?,
                    archived = 0,
                    updated_at = ?,
                    updated_at_ms = ?,
                    recency_at = ?,
                    recency_at_ms = ?
                WHERE id = ?
                """,
                (
                    str(rollout_path),
                    str(repo),
                    project_id,
                    now,
                    now_ms,
                    now,
                    now_ms,
                    sid,
                ),
            )

        else:
            title = (
                f"Imported Codex session {sid}"
            )

            preview = title

            conn.execute(
                """
                INSERT INTO threads (
                    id,
                    rollout_path,
                    created_at,
                    updated_at,
                    source,
                    model_provider,
                    cwd,
                    title,
                    sandbox_policy,
                    approval_mode,
                    tokens_used,
                    has_user_event,
                    archived,
                    cli_version,
                    first_user_message,
                    preview,
                    recency_at,
                    recency_at_ms,
                    history_mode,
                    project_id
                )
                VALUES (
                    ?, ?, ?, ?,
                    ?, ?, ?, ?,
                    ?, ?, ?, ?,
                    ?, ?, ?, ?,
                    ?, ?, ?, ?
                )
                """,
                (
                    sid,
                    str(rollout_path),
                    now,
                    now,
                    "vscode",
                    "openai",
                    str(repo),
                    title,
                    "workspace-write",
                    "on-request",
                    0,
                    1,
                    0,
                    "",
                    title,
                    preview,
                    now,
                    now_ms,
                    "legacy",
                    project_id,
                ),
            )

        conn.commit()

        print(
            "Codex Desktop thread registered"
        )
        print(
            f"Project:  {repo.name}"
        )
        print(
            f"Project ID: {project_id}"
        )

    finally:
        conn.close()


def resume_session():
    sid, imported_paths = import_session()

    register_thread_in_desktop(
        sid,
        imported_paths,
    )

    repo = git_root()

    print()
    print(f"Resuming {sid}")
    print()

    os.chdir(repo)

    os.execvp(
        "codex",
        [
            "codex",
            "resume",
            sid,
        ],
    )


def status():
    manifest = read_manifest()

    print(
        f"Session:  {manifest['session_id']}"
    )
    print(
        f"Exported: {manifest['exported_at']}"
    )
    print(
        f"From:     {manifest['source_repo']}"
    )
    print(
        f"Files:    {len(manifest.get('files', []))}"
    )

    for item in manifest.get(
        "files",
        [],
    ):
        print(
            f"  {item['relative_path']}"
        )


def list_sessions():
    root = sessions_root()

    for path in sorted(
        root.rglob("*.jsonl"),
        key=lambda p: p.stat().st_mtime_ns,
        reverse=True,
    ):
        try:
            with path.open(
                "r",
                encoding="utf-8",
                errors="replace",
            ) as f:
                for line in f:
                    try:
                        item = json.loads(line)
                    except json.JSONDecodeError:
                        continue

                    if (
                        item.get("type")
                        != "session_meta"
                    ):
                        continue

                    payload = item.get(
                        "payload",
                        {},
                    )

                    print()
                    print(
                        f"Session: "
                        f"{payload.get('id')}"
                    )
                    print(
                        f"CWD:     "
                        f"{payload.get('cwd')}"
                    )
                    print(
                        f"File:    {path}"
                    )

                    break

        except OSError:
            pass


def run_git(*args: str):
    result = subprocess.run(
        ["git", *args],
        cwd=git_root(),
        encoding="utf-8",
        errors="replace",
    )

    if result.returncode != 0:
        raise RuntimeError(
            f"git {' '.join(args)} "
            f"failed with code "
            f"{result.returncode}"
        )


def push_session():
    export_session()

    repo = git_root()

    run_git(
        "add",
        "-A",
    )

    status_result = subprocess.run(
        [
            "git",
            "diff",
            "--cached",
            "--quiet",
        ],
        cwd=repo,
    )

    if status_result.returncode == 0:
        print(
            "No changes to commit"
        )
    else:
        timestamp = (
            datetime.now()
            .strftime("%Y-%m-%d %H:%M")
        )

        run_git(
            "commit",
            "-m",
            f"Sync Codex session {timestamp}",
        )

    run_git("push")

    print()
    print("Codex session pushed")


def pull_session():
    run_git(
        "pull",
        "--rebase",
    )

    sid, imported_paths = import_session()

    register_thread_in_desktop(
        sid,
        imported_paths,
    )

    repo = git_root()

    print()
    print(f"Resuming {sid}")
    print()

    os.chdir(repo)

    os.execvp(
        "codex",
        [
            "codex",
            "resume",
            sid,
        ],
    )


def main():
    parser = argparse.ArgumentParser()

    parser.add_argument(
        "command",
        choices=[
            "export",
            "import",
            "resume",
            "status",
            "list",
            "push",
            "pull",
        ],
    )

    args = parser.parse_args()

    try:
        if args.command == "export":
            export_session()

        elif args.command == "import":
            sid, imported_paths = (
                import_session()
            )

            register_thread_in_desktop(
                sid,
                imported_paths,
            )

        elif args.command == "resume":
            resume_session()

        elif args.command == "status":
            status()

        elif args.command == "list":
            list_sessions()

        elif args.command == "push":
            push_session()

        elif args.command == "pull":
            pull_session()

    except Exception as exc:
        print(
            f"codex-sync: {exc}",
            file=sys.stderr,
        )
        sys.exit(1)


if __name__ == "__main__":
    main()