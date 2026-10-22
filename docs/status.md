# Stellar — Status / Active Questions

Tracking the ambiguities we're resolving from the reference program.
Decisions migrate to `docs/design.md` once agreed.

## Open decisions

### #6 — value vs pointer reads (stale reads)
**Question:** a read-only method passed a *value* gets a copy; if the original is
mutated elsewhere, can the read-only method observe *stale* data?

**Working answer:** not stale — *snapshot*. A value receiver receives an independent
copy at call time; it intentionally sees a snapshot, not live data. If a method
needs to observe the *current* value, it takes a pointer. The type system already
answers "what mutates / what doesn't": value = snapshot, pointer = shared mutable.

**Subtle issue still open:** shallow copies of types holding slices/maps/
pointers share underlying storage (partial aliasing). Decisions pending:
- (a) Are value copies **shallow** (default) or **deep**?
- (b) Do callers take pointers for reads on large/shared containers instead of
  relying on snapshot copies?

### #7 — Result vs Maybe
Both channels let the caller supply a default (`try x else v` / `try x else |e| v`),
so the practical difference is *intent*:
- `Result[T]` — failure is a **meaningful, handleable** reason (I/O, parse, validation).
- `Maybe[T]` — absence is a **normal expected** outcome, not an error (lookup miss).
**Open:** is out-of-range indexing a `Maybe` (not found) or `Result` (error)?
**Recommended:** unify on ONE channel to avoid duplicate `try` sugar — pick `Result`
and treat indexing-out-of-range as an error, unless the user insists Maybe is a
first-class "optional" type. Decide the `Some/None` vs `Ok/Err` convention accordingly.

### #error — Error interface mechanism
- How does `implements Error` auto-implement the interface? What fields/methods does
  `Error` require (e.g. `message: str`)?
- Is there a builtin `error_T` created via `errors.new()`?
- Are user errors captureable in `try … else |e|` and matchable?

### #commas — trailing commas [RESOLVED 2026-10-21]
**Decision:** trailing commas are optional in every comma-separated list.
`;` is a terminator (required after every statement, including the last
one in a block) except after an item ending in `}`. Switch arms follow the same
`}` rule: a comma is optional after a block arm, required between expression arms.

**Original question:** should a trailing comma be allowed (or required, or rejected)
consistently in every comma-separated list?

**Current parser behaviour (inconsistent):**
- Accepted: object fields, enum members, enum variant fields, switch arms
  (expression arms *require* one, block arms make it optional).
- Rejected: call arguments, function parameters, function-type parameters,
  sequence literals, type arguments, type parameters in `fn name[...]`.

Relevant because multi-line calls (e.g. `strings.format(...)` in the reference
program) break as soon as someone adds a trailing comma.
