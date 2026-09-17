#!/usr/bin/env python3
"""Read-only inventory of the canonical TV sample library, independent of Go.

This checks the sample's separated `S01E01[-E02]` naming, not every naming
variant supported by Igloo. Unrecognized visible videos require manual review.
Optionally reconcile every accepted path and episode list with the verbose
TestSampleLibraryReadOnly log. No media is opened or changed by this script.
"""

import argparse
from collections import Counter, defaultdict
import json
import os
from pathlib import Path
import re


def inventory(root):
    extensions = {".mkv", ".mp4", ".avi", ".mov", ".m4v", ".webm"}
    naming = re.compile(r"^.+[ ._-]S(\d+)E(\d+)(?:-E(\d+))?[ ._-].+$", re.I)
    accepted = {}
    excluded = defaultdict(list)
    shows = defaultdict(lambda: {"files": 0, "combined_files": 0, "links": 0})
    episodes = defaultdict(list)
    seasons = defaultdict(set)
    sizes = 0

    def walk_error(error):
        raise error

    for directory, directories, files in os.walk(root, onerror=walk_error):
        for name in directories:
            path = Path(directory, name)
            if path.is_symlink():
                excluded["symlink_directories"].append(str(path.relative_to(root)))
        for name in files:
            path = Path(directory, name)
            relative = path.relative_to(root)
            key = str(relative)
            video = path.suffix.lower() in extensions
            if any(part.startswith(".") for part in relative.parts):
                excluded["hidden_videos" if video else "hidden_other"].append(key)
                continue
            if not video:
                excluded["sidecars_or_other"].append(key)
                continue
            if path.is_symlink() or not path.is_file():
                excluded["special_files_for_review"].append(key)
                continue
            match = naming.fullmatch(path.stem)
            season = re.fullmatch(r"Season (\d+)", relative.parts[1], re.I) if len(relative.parts) == 3 else None
            if match is None or season is None:
                excluded["unrecognized_videos_for_review"].append(key)
                continue
            number, first, last = match.groups()
            number, first, last = int(number), int(first), int(last or first)
            if number != int(season[1]) or first <= 0 or last < first:
                excluded["invalid_numbering"].append(key)
                continue
            numbers = list(range(first, last + 1))
            accepted[key] = numbers
            show = relative.parts[0]
            shows[show]["files"] += 1
            shows[show]["combined_files"] += len(numbers) > 1
            shows[show]["links"] += len(numbers)
            seasons[show].add(number)
            for episode in numbers:
                episodes[(show, number, episode)].append(key)
            sizes += path.stat().st_size

    duplicates = []
    for (show, season, episode), paths in sorted(episodes.items()):
        if len(paths) > 1:
            duplicates.append({"show": show, "season": season, "episode": episode, "files": sorted(paths)})
    for show, counts in shows.items():
        counts["seasons"] = len(seasons[show])
        counts["episodes"] = sum(key[0] == show for key in episodes)
    totals = Counter()
    for counts in shows.values():
        totals.update(counts)
    totals["shows"] = len(shows)
    totals["bytes"] = sizes
    return {
        "totals": dict(totals),
        "shows": dict(shows),
        "duplicate_episode_copies": duplicates,
        "excluded_counts": {key: len(paths) for key, paths in excluded.items()},
        "excluded": {key: sorted(paths) for key, paths in excluded.items()},
        "accepted": dict(sorted(accepted.items())),
    }


def reconcile(root, report, log_path):
    observed = {}
    log = log_path.read_text()
    for line in log.splitlines():
        if "catalog file: " not in line:
            continue
        value = line.split("catalog file: ", 1)[1]
        path, end = json.JSONDecoder().raw_decode(value)
        numbers = re.fullmatch(r" episodes: \[([\d ]*)\]", value[end:])
        if numbers is None:
            raise ValueError(f"Unrecognized catalog log entry: {line}")
        key = str(Path(path).relative_to(root))
        if key in observed:
            raise ValueError(f"Duplicate catalog path: {key}")
        observed[key] = [int(number) for number in numbers[1].split()]
    expected = report["accepted"]
    differences = {
        "missing_paths": sorted(expected.keys() - observed.keys()),
        "unexpected_paths": sorted(observed.keys() - expected.keys()),
        "episode_mismatches": sorted(key for key in expected.keys() & observed.keys() if expected[key] != observed[key]),
    }
    passed = "--- PASS: TestSampleLibraryReadOnly (" in log
    report["reconciliation"] = {"sample_test_passed": passed, **differences}
    return passed and bool(expected) and not any(differences.values())


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("root", type=Path)
    parser.add_argument("--scan-log", type=Path)
    args = parser.parse_args()
    root = args.root.resolve(strict=True)
    if not root.is_dir():
        parser.error("root must be a readable directory")
    report = inventory(root)
    success = True
    if args.scan_log:
        success = reconcile(root, report, args.scan_log)
    print(json.dumps(report, indent=2, sort_keys=True))
    raise SystemExit(0 if success else 1)
