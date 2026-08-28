# Exact prompts

Each heading is one initial user turn. Text after **Revision** is sent only when
the scenario's trigger reports that at least one Producer is running.

## three-independent

In the supplied fixture, fix the misspelled CLI help text, add the missing
accessible label to the settings close button, and correct the documented
default timeout. Validate and deliver all three changes.

## broad-research

Using only the supplied public source pack, recommend one migration strategy.
Cover compatibility, operational risk, rollback, cost, and evidence quality;
return one coherent recommendation rather than separate drafts.

## fault-diagnosis

Diagnose the intermittent duplicate-session incident in the supplied service.
Evaluate at least the persistence, retry/idempotency, scheduling, and UI
projection hypotheses, then identify the best-supported root cause and test it.

## mutually-exclusive-design

Choose between event sourcing and transactional snapshots for the supplied
offline-first feature. The answer must select one route, explain the rejected
route, give migration and rollback plans, and preserve the listed invariants.

## tightly-serial

Apply the three ordered schema migrations in the fixture. Migration two depends
on data produced by migration one, and migration three depends on migration
two's checksum. Do not parallelize steps that cannot be independently accepted.

## competing-files

Produce two genuinely different implementations of the parser correction in
the supplied repository. Both routes modify the same parser and test files, so
isolate them, compare evidence, and integrate exactly one canonical result.

## partial-producer-failure

Investigate the supplied compatibility regression using independent browser,
storage, and API hypotheses. One fixture route intentionally fails to start;
continue when the remaining evidence is sufficient and do not mark the whole
Work failed solely because that route failed.

## mid-flight-goal-revision

Implement the supplied export feature with the current CSV-only requirement.

**Revision:** The goal has changed: JSON Lines is now required and CSV must not
be delivered. Invalidate stale routes, preserve an honest audit trail, and
deliver only a candidate for the revised goal.
