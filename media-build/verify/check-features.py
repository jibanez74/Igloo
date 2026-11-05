"""Check an ffmpeg build against required-features.txt, optionally diffing it
against a reference build (for example the Jellyfin payload).

    python3 -I check-features.py required-features.txt <ffmpeg> [<reference ffmpeg>]

Exits 1 when a required component is missing.
"""

import subprocess
import sys

# kind -> ffmpeg list option
LISTS = {
    "encoder": "-encoders",
    "decoder": "-decoders",
    "muxer": "-muxers",
    "demuxer": "-demuxers",
    "filter": "-filters",
    "hwaccel": "-hwaccels",
    "protocol": "-protocols",
    "indev": "-devices",
    "outdev": "-devices",
    "bsf": "-bsfs",
}


def run_list(ffmpeg, option):
    out = subprocess.run(
        [ffmpeg, "-hide_banner", option],
        capture_output=True, text=True, timeout=30, check=True,
    ).stdout
    return out.splitlines()


def table_rows(lines):
    """Rows after the dashed separator: (flags, names)."""
    rows, started = [], False
    for line in lines:
        if not started:
            started = line.strip().startswith("---")
            continue
        fields = line.split()
        if len(fields) >= 2:
            rows.append((fields[0], fields[1].split(",")))
    return rows


def inventory(ffmpeg):
    have = {kind: set() for kind in LISTS}
    for kind in ("encoder", "decoder", "filter", "muxer", "demuxer"):
        for _, names in table_rows(run_list(ffmpeg, LISTS[kind])):
            have[kind].update(names)
    for flags, names in table_rows(run_list(ffmpeg, "-devices")):
        if "D" in flags:
            have["indev"].update(names)
        if "E" in flags:
            have["outdev"].update(names)
    for kind in ("hwaccel", "bsf"):
        lines = run_list(ffmpeg, LISTS[kind])[1:]
        have[kind].update(line.strip() for line in lines if line.strip())
    for line in run_list(ffmpeg, "-protocols")[1:]:
        name = line.strip()
        if name and not name.endswith(":"):
            have["protocol"].add(name)
    return have


def main():
    if len(sys.argv) not in (3, 4):
        sys.exit(__doc__)
    required_file, candidate = sys.argv[1], sys.argv[2]
    reference = sys.argv[3] if len(sys.argv) == 4 else None

    have = inventory(candidate)
    missing = []
    with open(required_file, encoding="utf-8") as f:
        for line in f:
            line = line.split("#", 1)[0].strip()
            if not line:
                continue
            kind, name = line.split()
            if kind not in LISTS:
                sys.exit(f"unknown kind {kind!r} in {required_file}")
            if name not in have[kind]:
                missing.append(f"{kind} {name}")

    if reference:
        ref = inventory(reference)
        for kind in LISTS:
            gone = sorted(ref[kind] - have[kind])
            added = sorted(have[kind] - ref[kind])
            print(f"{kind}: {len(have[kind])} (reference {len(ref[kind])})")
            if gone:
                print(f"  only in reference ({len(gone)}): {' '.join(gone)}")
            if added:
                print(f"  only in candidate ({len(added)}): {' '.join(added)}")

    if missing:
        print(f"MISSING {len(missing)} required component(s):")
        for item in missing:
            print(f"  {item}")
        sys.exit(1)
    print("all required components present")


if __name__ == "__main__":
    main()
