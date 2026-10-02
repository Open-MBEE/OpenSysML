# Agent skills for using OpenSysML

These skills teach an LLM coding agent to *use* OpenSysML: write SysML v2 models it accepts, check
and run them with the `sysml` command, and drive them from Python. They are for people modeling with
OpenSysML. The skills under `.agents/skills/` at the repository root are a different set, for agents
working *on* OpenSysML itself.

| Skill | Use it to |
|-------|-----------|
| [`sysml-v2-modeling`](skills/sysml-v2-modeling/SKILL.md) | write SysML v2 textual notation that validates and runs |
| [`opensysml-cli`](skills/opensysml-cli/SKILL.md) | validate, evaluate, run behavior, check requirements and convert models with `sysml` |
| [`opensysml-python`](skills/opensysml-python/SKILL.md) | do the same from a Python script through the `opensysml` client |

Each skill is a directory holding a `SKILL.md` with `name` and `description` front matter, the
[Agent Skills](https://agentskills.io) layout. To install them, copy or symlink the directories under
`skills/` into the skills directory your agent reads, either per project (for example
`<project>/.agents/skills/`) or per user. The skills assume `sysml` is on `PATH`
([installation](../../docs/guide/01-install.md)) and, for the Python skill, `pip install opensysml`.
