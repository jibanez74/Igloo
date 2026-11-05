"""Compare two ffprobe builds on real media, field by field, limited to what
Igloo reads (server/cmd/internal/ffprobe/ffprobe_metadata.go).

    python3 -I ffprobe-parity.py <reference ffprobe> <candidate ffprobe> <report.json> <dir>...

Video files get GetMetadata's arguments and audio files GetAudioMetadata's,
chosen by the extensions the scanners accept (helpers/media_types.go). Prints
a summary and writes every differing file to the report. Media is only read.
"""

import concurrent.futures
import json
import os
import re
import subprocess
import sys

VIDEO_EXT = {".mp4", ".m4v", ".mov", ".mkv", ".webm", ".avi"}
AUDIO_EXT = {".mp3", ".flac", ".m4a"}

VIDEO_ARGS = ["-v", "quiet", "-print_format", "json", "-show_streams", "-show_format", "-show_chapters"]
AUDIO_ARGS = [
    "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams",
    "-show_entries",
    "format=duration,bit_rate:format_tags:stream=codec_name,codec_type,profile,channels,channel_layout:stream_tags",
]

STREAM_FIELDS = [
    "index", "codec_name", "codec_type", "profile", "bit_rate", "sample_rate", "channels",
    "channel_layout", "width", "height", "coded_width", "coded_height", "display_aspect_ratio",
    "level", "avg_frame_rate", "r_frame_rate", "bits_per_raw_sample", "pix_fmt", "color_range",
    "color_transfer", "color_primaries", "color_space", "field_order",
]
DISPOSITIONS = ["attached_pic", "default", "forced"]
# Tag lookups are case- and separator-insensitive, so compare normalized keys.
STREAM_TAGS = {"title", "language", "lang"}
FORMAT_TAGS = {
    "title", "artist", "albumartist", "composer", "album", "genre", "track", "tracknumber",
    "disc", "discnumber", "date", "year", "copyright", "sortname", "titlesort", "sortalbum",
    "albumsort", "sortartist", "artistsort", "language", "lang",
}


def norm_key(key):
    return re.sub(r"[^a-z0-9]", "", key.lower())


def pick_tags(tags, wanted):
    out = {}
    for key, value in (tags or {}).items():
        k = norm_key(key)
        if k in wanted and isinstance(value, str) and value:
            out.setdefault(k, []).append(value)
    return {k: sorted(v) for k, v in sorted(out.items())}


def consumed(result):
    streams = []
    for s in result.get("streams", []):
        entry = {f: s.get(f) for f in STREAM_FIELDS}
        entry["rotation"] = [
            sd.get("rotation") for sd in s.get("side_data_list", [])
            if sd.get("side_data_type") == "Display Matrix"
        ]
        entry["disposition"] = {d: (s.get("disposition") or {}).get(d) for d in DISPOSITIONS}
        entry["tags"] = pick_tags(s.get("tags"), STREAM_TAGS)
        streams.append(entry)
    fmt = result.get("format", {})
    return {
        "streams": streams,
        "format": {
            "duration": fmt.get("duration"),
            "start_time": fmt.get("start_time"),
            "bit_rate": fmt.get("bit_rate"),
            "tags": pick_tags(fmt.get("tags"), FORMAT_TAGS),
        },
        "chapters": [
            {"start_time": c.get("start_time"), "title": (c.get("tags") or {}).get("title")}
            for c in result.get("chapters", [])
        ],
    }


def probe(ffprobe, args, path):
    try:
        proc = subprocess.run([ffprobe, *args, path], capture_output=True, timeout=60)
    except subprocess.TimeoutExpired:
        return {"error": "timeout"}
    if proc.returncode != 0:
        return {"error": f"exit {proc.returncode}"}
    try:
        return consumed(json.loads(proc.stdout))
    except json.JSONDecodeError as exc:
        return {"error": f"bad json: {exc}"}


def diff(a, b, path=""):
    if isinstance(a, dict) and isinstance(b, dict):
        out = []
        for key in sorted(set(a) | set(b)):
            out += diff(a.get(key), b.get(key), f"{path}.{key}" if path else key)
        return out
    if isinstance(a, list) and isinstance(b, list) and len(a) == len(b) and a and isinstance(a[0], dict):
        out = []
        for i, (x, y) in enumerate(zip(a, b)):
            out += diff(x, y, f"{path}[{i}]")
        return out
    return [] if a == b else [(path, a, b)]


def compare(reference, candidate, path):
    ext = os.path.splitext(path)[1].lower()
    args = VIDEO_ARGS if ext in VIDEO_EXT else AUDIO_ARGS
    ref, cand = probe(reference, args, path), probe(candidate, args, path)
    return path, diff(ref, cand), "error" in ref, "error" in cand


def media_files(roots):
    for root in roots:
        for dirpath, _, names in os.walk(root, followlinks=False):
            for name in sorted(names):
                ext = os.path.splitext(name)[1].lower()
                if ext in VIDEO_EXT or ext in AUDIO_EXT:
                    yield os.path.join(dirpath, name)


def main():
    if len(sys.argv) < 5:
        sys.exit(__doc__)
    reference, candidate, report_path, roots = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
    files = sorted(media_files(roots))
    differing, field_counts = {}, {}
    # A file both builds fail to probe compares equal, so failures are counted
    # separately rather than hidden in the "no differences" total.
    failures = {"reference": [], "candidate": []}
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        for path, changes, ref_failed, cand_failed in pool.map(
            lambda p: compare(reference, candidate, p), files
        ):
            if ref_failed:
                failures["reference"].append(path)
            if cand_failed:
                failures["candidate"].append(path)
            if not changes:
                continue
            differing[path] = [{"field": f, "reference": a, "candidate": b} for f, a, b in changes]
            for field, _, _ in changes:
                generic = re.sub(r"\[\d+\]", "[]", field)
                field_counts[generic] = field_counts.get(generic, 0) + 1
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(
            {"files": len(files), "differing": differing, "failures": failures},
            f, indent=2, sort_keys=True,
        )
    print(f"{len(files)} files probed, {len(differing)} with differences")
    print(
        f"probe failures: reference {len(failures['reference'])}, "
        f"candidate {len(failures['candidate'])}"
    )
    for field, count in sorted(field_counts.items(), key=lambda kv: -kv[1]):
        print(f"  {count:5d}  {field}")
    print(f"report: {report_path}")


if __name__ == "__main__":
    main()
