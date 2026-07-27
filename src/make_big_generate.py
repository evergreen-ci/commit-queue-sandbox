#!/usr/bin/env python3
"""Build a large, sandbox-safe generate.tasks payload from a real generated config.

The source config (e.g. a mongodb-mongo-master generate.tasks payload) references
distros, functions, and compile tasks that do not exist in this project, so it
cannot be fed to generate.tasks as-is. This script rewrites it:

  * every task's commands are replaced with a trivial shell.exec
  * build variants are emitted as *new* variants (display_name + run_on added)
  * distros are replaced with a distro that exists here
  * dependencies pointing outside the generated set are dropped
  * everything is emitted with activate: false so nothing is actually scheduled

Scale knobs (--variants / --tasks-per-variant) let you dial the payload up or
down when measuring generate.tasks performance.
"""

import argparse
import json
import sys

DEFAULT_DISTRO = "ubuntu2004-small"
NOOP_COMMANDS = [
    {
        "command": "shell.exec",
        "params": {"script": "echo ${task_name}"},
    }
]


def parse_args(argv):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", help="path to a real generate.tasks JSON payload")
    parser.add_argument(
        "-o", "--output", default="src/generate_big.json", help="output path"
    )
    parser.add_argument(
        "--variants", type=int, default=None, help="keep at most N build variants"
    )
    parser.add_argument(
        "--tasks-per-variant",
        type=int,
        default=None,
        help="keep at most N tasks per build variant",
    )
    parser.add_argument(
        "--distro", default=DEFAULT_DISTRO, help="distro to run the generated tasks on"
    )
    parser.add_argument(
        "--prefix",
        default="",
        help="prefix added to every generated variant and task name",
    )
    parser.add_argument(
        "--keep-commands",
        action="store_true",
        help="keep the original commands instead of replacing them with a no-op",
    )
    return parser.parse_args(argv)


def build(source, args):
    variants = source["buildvariants"]
    if args.variants is not None:
        variants = variants[: args.variants]

    def bv_name(name):
        return args.prefix + name

    def task_name(name):
        return args.prefix + name

    source_tasks = {t["name"]: t for t in source["tasks"]}

    kept_variants = []
    kept_task_names = set()
    for variant in variants:
        bv_tasks = [t for t in variant.get("tasks", []) if t["name"] in source_tasks]
        if args.tasks_per_variant is not None:
            bv_tasks = bv_tasks[: args.tasks_per_variant]
        if not bv_tasks:
            continue
        kept_variants.append((variant, bv_tasks))
        kept_task_names.update(t["name"] for t in bv_tasks)

    variant_names = {v["name"] for v, _ in kept_variants}

    def clean_deps(deps):
        cleaned = []
        for dep in deps or []:
            if not isinstance(dep, dict):
                continue
            if dep.get("name") not in kept_task_names:
                continue
            # A dependency with no variant resolves to the depending task's own
            # variant, which is always in the generated set.
            if "variant" in dep and dep["variant"] not in variant_names:
                continue
            new_dep = {"name": task_name(dep["name"])}
            if "variant" in dep:
                new_dep["variant"] = bv_name(dep["variant"])
            cleaned.append(new_dep)
        return cleaned

    out_variants = []
    for variant, bv_tasks in kept_variants:
        out_tasks = []
        for bv_task in bv_tasks:
            entry = {"name": task_name(bv_task["name"]), "activate": False}
            deps = clean_deps(bv_task.get("depends_on"))
            if deps:
                entry["depends_on"] = deps
            if "distros" in bv_task:
                entry["distros"] = [args.distro]
            out_tasks.append(entry)

        out_variant = {
            "name": bv_name(variant["name"]),
            "display_name": bv_name(variant["name"]),
            "run_on": [args.distro],
            "activate": False,
            "tasks": out_tasks,
        }

        emitted = {t["name"] for t in out_tasks}
        display_tasks = []
        for display_task in variant.get("display_tasks", []) or []:
            execution_tasks = [
                task_name(name)
                for name in display_task.get("execution_tasks", [])
                if task_name(name) in emitted
            ]
            if execution_tasks:
                display_tasks.append(
                    {
                        "name": bv_name(display_task["name"]),
                        "execution_tasks": execution_tasks,
                    }
                )
        if display_tasks:
            out_variant["display_tasks"] = display_tasks

        out_variants.append(out_variant)

    out_tasks = []
    for name in sorted(kept_task_names):
        task = source_tasks[name]
        out_task = {
            "name": task_name(name),
            "commands": task["commands"] if args.keep_commands else NOOP_COMMANDS,
        }
        deps = clean_deps(task.get("depends_on"))
        if deps:
            out_task["depends_on"] = deps
        out_tasks.append(out_task)

    return {"buildvariants": out_variants, "tasks": out_tasks}


def main(argv=None):
    args = parse_args(argv if argv is not None else sys.argv[1:])
    with open(args.source) as f:
        source = json.load(f)

    generated = build(source, args)

    with open(args.output, "w") as f:
        json.dump(generated, f)

    pairs = sum(len(v["tasks"]) for v in generated["buildvariants"])
    print(
        f"{args.output}: {len(generated['buildvariants'])} variants, "
        f"{len(generated['tasks'])} tasks, {pairs} variant/task pairs"
    )


if __name__ == "__main__":
    main()
