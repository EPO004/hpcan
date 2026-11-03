#!/usr/bin/env python3
"""Run identical OT/AC/DUP/GRIT timing configurations and preserve provenance."""
import argparse
import csv
import itertools
import json
import pathlib
import subprocess
import sys


def git(repo, *args):
    return subprocess.check_output(["git", "-C", str(repo), *args], text=True).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=pathlib.Path)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--gpus", default="1,2,3,4")
    parser.add_argument("--timeout", type=float, default=1800)
    parser.add_argument("--ablations", action="store_true")
    argv = sys.argv[1:]
    if "--" in argv:
        split = argv.index("--")
        options, extra = argv[:split], argv[split + 1:]
    else:
        options, extra = argv, []
    args = parser.parse_args(options)
    repo = pathlib.Path(__file__).resolve().parents[2]
    binary = args.binary.resolve(strict=True)
    args.output.mkdir(parents=True, exist_ok=False)
    metadata = {"mgpusim": git(repo, "rev-parse", "HEAD"),
                "akita": git(repo.parent / "akita", "rev-parse", "HEAD"),
                "mgpusim_status": git(repo, "status", "--short"),
                "akita_status": git(repo.parent / "akita", "status", "--short"),
                "go": subprocess.check_output(["go", "version"], text=True).strip(),
                "binary": str(binary)}
    (args.output / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    runs = [(mode, mode, True, True) for mode in ("ot", "ac", "dup", "grit")]
    if args.ablations:
        runs += [(f"pa-cache-{cache}-prediction-{predict}", "grit", cache, predict)
                 for cache, predict in itertools.product((False, True), repeat=2)
                 if not (cache and predict)]
    reports = []
    for name, mode, cache, predict in runs:
        prefix = (args.output / name).resolve()
        command = [str(binary), "-timing", "-verify", "-disable-rtm",
                   "-use-unified-memory", f"-unified-gpus={args.gpus}",
                   f"-grit-mode={mode}", f"-grit-pa-cache={str(cache).lower()}",
                   f"-grit-prediction={str(predict).lower()}",
                   f"-metric-file-name={prefix}", *extra]
        (args.output / f"{name}.command.json").write_text(json.dumps(command, indent=2) + "\n")
        with (args.output / f"{name}.log").open("w") as log:
            subprocess.run(command, stdout=log, stderr=subprocess.STDOUT,
                           check=True, timeout=args.timeout)
        report = json.loads(pathlib.Path(f"{prefix}.grit.json").read_text())
        if report["config"]["Mode"] != mode:
            raise RuntimeError(f"{name}: benchmark arguments overrode the requested policy")
        if report["active_requests"] or report["pending_operations"]:
            raise RuntimeError(f"{name}: simulator stopped with unfinished GRIT work")
        elapsed = report["kernel_time_ns"]
        if elapsed <= 0:
            raise RuntimeError(f"{name}: kernel timing is missing")
        reports.append((name, elapsed, report))
    baseline = reports[0][1]
    with (args.output / "comparison.csv").open("w", newline="") as out:
        writer = csv.writer(out)
        writer.writerow(["mode", "kernel_time_ns", "speedup_vs_ot", "improvement_percent",
                         "local_faults", "protection_faults", "migrations", "copies", "collapses"])
        for name, elapsed, report in reports:
            faults, placement = report["faults"], report["placement"]
            speedup = baseline / elapsed
            writer.writerow([name, elapsed, speedup, (speedup - 1) * 100,
                             faults["LocalPageFaults"], faults["ProtectionFaults"],
                             placement["Migrations"], placement["Copies"], placement["Collapses"]])
    print(args.output / "comparison.csv")


if __name__ == "__main__":
    main()
