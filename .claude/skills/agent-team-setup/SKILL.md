---
name: agent-team-setup
description: Guides users through setting up an agent team from scratch - project definition, team design, conventions - then generates all .claude specs. Use when the user wants to create a new agent team project.
---

# Agent Team Setup

Walk the user through setting up a complete agent team project, then generate all
the Claude specs from it.

## When to use

When the user wants to create a new agent team project from scratch, or regenerate
specs from existing definition files.

## How it works

Four phases, worked through in order. Each phase produces a definition file. Once all
phases are complete, a single generation step creates all the Claude specs.

Templates for each phase are in `templates/`. Completed examples are in `examples/`.
Design principles are in `principles.md`.

When walking the user through each phase, reference the examples to show what a
filled-out file looks like.

### Phase 1: Project (what are we building?)

Walk the user through filling out a PROJECT.md file using the template in
`templates/PROJECT.md`.

Keep it minimal. Offer a draft for the user to react to.

Output: `PROJECT.md`

### Phase 2: Team (who does what?)

Walk the user through filling out a TEAM.md file using the template in
`templates/TEAM.md`. Work through each section in order, offering a first draft
for the user to react to rather than asking open-ended questions.

The sections, in order:

1. **Roles** - define before workflow, since workflow states map to role owners
2. **Workflow** - the states work moves through, each owned by a role
3. **Handoffs** - contracts between roles defining what the artifact looks like when passed
4. **Rules** - team-wide rules first, then per-role rules

Output: `TEAM.md`

### Phase 3: Conventions (how do we work?)

Walk the user through filling out a CONVENTIONS.md file using the template in
`templates/CONVENTIONS.md`. Skip sections the user says don't apply. Provide sensible
defaults for universal items, leave genuinely project-specific items for the user to
fill in.

Output: `CONVENTIONS.md`

### Phase 4: Generate (create all the specs)

Once the definition files are complete, generate all Claude specs in one step:

1. **CLAUDE.md** from PROJECT.md + TEAM.md + CONVENTIONS.md:
   - Project name and description
   - Tech stack
   - Team roles summary (role name + what it owns, one line each)
   - Workflow state table
   - List of available skills
   - Note that conventions are in `.claude/rules/` files

2. **`.claude/rules/team.md`** from TEAM.md:
   - Team-wide rules

3. **`.claude/rules/*.md`** from CONVENTIONS.md:
   - Group related conventions into rule files by domain
   - Git -> `.claude/rules/git.md`
   - Code, Dependencies -> `.claude/rules/code.md`
   - Testing -> `.claude/rules/testing.md`
   - Infrastructure, Security -> `.claude/rules/infrastructure.md`
   - Documentation -> `.claude/rules/documentation.md`
   - Collapse small sections into related files

4. **`.claude/skills/{role}/SKILL.md`** for each role in TEAM.md:
   - The role's responsibilities and boundaries
   - The handoff contracts where this role is sender or receiver
   - The per-role rules
   - The workflow states this role owns, as step-by-step instructions

Keep the definition files (PROJECT.md, TEAM.md, CONVENTIONS.md) in the repo as source
of truth. If something needs to change, update the definition file and regenerate.
