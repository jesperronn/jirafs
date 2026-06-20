# Orchestration Model

## Goal

Development should be decomposed so one orchestrator can coordinate many small
builders without each builder needing broad project context.

The orchestrator owns sequencing, scope boundaries, and integration proof.
Builders own one small implementation step at a time.

Read this together with:

- [Implementation Roadmap](implementation-roadmap.md)
- [Implementation Packets](implementation-packets.md)
- [Parallel Workstreams](parallel-workstreams.md)
- [Development Rules](development-rules.md)
- [Validator Contract](validator-contract.md)

## Roles

### Orchestrator

The orchestrator is the only role allowed to hold the whole plan in memory.

The orchestrator must:

- break roadmap work into packets and sub-packets
- choose the next smallest independent task
- assign exact path ownership and explicit non-goals
- define acceptance criteria before coding starts
- decide whether a task is a leaf task or an integration task
- collect verification evidence from builders
- reject incomplete or scope-creep handoffs
- compose validated builder outputs into a coherent branch

The orchestrator must not:

- hand a builder an architecture-sized task
- ask a builder to "just keep going" without a stop condition
- merge work that lacks the required validator evidence
- allow multiple builders to edit the same ownership zone without an explicit
  integration task

### Builder

A builder should receive one narrow task with one obvious proof target.

A builder must:

- stay inside the assigned paths
- implement only the requested seam or behavior
- run the required local validation loop for that task
- record exact commands and outcomes
- stop and hand back when the acceptance criteria are met or blocked

A builder must not:

- redesign adjacent contracts without escalation
- perform opportunistic cleanup outside scope
- continue into the next packet after finishing the assigned one
- claim integration safety outside the owned slice

## Decomposition Standard

The orchestrator should decompose work in this order:

1. Freeze the contract.
2. Split by ownership boundary.
3. Split each boundary into the smallest proof-bearing unit.
4. Assign builders only one proof-bearing unit at a time.
5. Re-run validation at the integration boundary.

Good builder tasks are:

- add one model and its tests
- add one parser rule and its golden files
- add one CLI command shell with fixed output contract
- add one resolver failure mode with regression coverage
- refactor one module behind unchanged behavior and prove no regression

Bad builder tasks are:

- implement planning
- build sync
- clean up the codebase
- make the CLI nicer everywhere
- fix all validation issues

## Standard Task Envelope

Every builder task should fit this envelope:

- one objective
- one ownership zone
- one acceptance checklist
- one validator checklist
- one explicit stop condition

The orchestrator should prefer tasks that can usually be completed in one short
edit loop and reviewed from one small diff.

## Handoff Format

Every builder handoff should use this structure:

```text
Task:
- one-sentence objective

Scope:
- owned paths
- explicit out-of-scope paths

Acceptance:
- concrete behaviors now true

Validation:
- exact commands run
- pass/fail result for each command
- manual checks performed

Files changed:
- exact paths

Status:
- done, partial, or blocked

Next smallest step:
- one follow-on action for the next builder or orchestrator

Risks:
- open questions, contract pressure, or known gaps
```

## Delegation Rules

The orchestrator should delegate only when all of the following are true:

- the task has one clear owner
- the input contract is stable enough for isolated work
- the output can be validated without broad system knowledge
- failure can be reported without leaving the tree ambiguous

Keep the orchestrator work for:

- cross-packet sequencing
- contract changes
- multi-stream integration
- merge conflict resolution
- final verification before review

Delegate to builders:

- path-local implementation
- path-local refactors
- targeted test additions
- fixture authoring
- doc updates tied to one local contract

## Builder Task Template

Use this prompt shape for small builders:

```text
Objective:
Implement <one narrow behavior>.

Owned paths:
<exact paths>

Do not edit:
<exact paths or ownership zones>

Acceptance criteria:
<flat list of observable outcomes>

Required validation:
<exact commands>

Stop when:
<clear finish line>

Report back with:
<handoff format fields>
```

## Integration Rule

Builder-complete does not mean merge-ready.

A change becomes merge-ready only after the orchestrator confirms:

- the builder stayed inside scope
- the validator evidence matches the final diff
- the work still fits the roadmap and contract docs
- any cross-stream effects are reconciled

## Default Standard

- orchestrator owns planning and integration
- builders own one small proof-bearing task each
- handoffs must be structured and replayable
- integration happens only after validator evidence is reviewed
