#!/usr/bin/env python3
"""Run all checks, excluding only the three explicitly accepted driver specs."""
import argparse
import pathlib
import subprocess

SKIP = "|".join([
    "should handle RDMA Drain RSP",
    "should process page migration rsp from CP and send restart reqs to GPU and reply to MMU",
    "should process GPU restart rsp and send restart req to RDMAs",
])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--race", action="store_true")
    args = parser.parse_args()
    repo = pathlib.Path(__file__).resolve().parents[2]
    packages = subprocess.check_output(["go", "list", "./..."], cwd=repo, text=True).split()
    packages = [p for p in packages if p != "github.com/sarchlab/mgpusim/v3/driver"]
    commands = [(repo.parent / "akita", ["go", "test", "./...", "-timeout", "90s"]),
                (repo, ["go", "test", "-timeout", "90s", *packages]),
                (repo, ["go", "test", "./driver", "-timeout", "45s", "-ginkgo.skip=" + SKIP]),
                (repo, ["go", "vet", "./grit/...", "./samples/runner", "./driver/internal",
                        "./timing/pagemigrationcontroller", "./timing/rdma"])]
    if args.race:
        commands.append((repo,["go","test","-race","./grit/..."]))
        commands.append((repo, ["go", "test", "-race", "./samples/runner",
                                "-run", "TestGRIT", "-timeout", "120s"]))
    for directory, command in commands:
        subprocess.run(command, cwd=directory, check=True)


if __name__ == "__main__":
    main()
