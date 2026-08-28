# AI Assistance Policy

This repository is built with AI assistance, and says so because SPEC §1.1 requires a
claim to name its witness. A codebase whose central argument is that provenance must be
recorded, and which does not record its own, is making an exception for itself.

## Three rules

**Review on observable quality, never on whether the output looks generated.** A
reviewer's job is to find defects, and "this reads like a model wrote it" is not one.
The inverse matters more: prose that reads as though a person wrote it is not thereby
correct, and a review that spends its attention on style has less left for the
reasoning. Every check in this repository is designed on the same principle — §17
refuses to score, and §9.4 validates a quotation against archived bytes rather than
against how confident it sounds.

**Reject what the contributor cannot explain or defend.** This is the rule with teeth,
and it is the reason the first rule is safe. Assistance is unlimited; accountability is
not transferable. If you cannot say why a function refuses in the case it refuses, why a
threshold is the number it is, or what breaks if a check is removed, the change is not
ready — whoever or whatever wrote it. §10.6.4 makes the same bet one level up: a
required rationale filters more bad adjudications than a permission check, because
somebody who cannot articulate a reason usually stops before finishing the sentence.

**No human attribution trailers for tools.** A `Co-authored-by` naming a model asserts
that something which cannot be asked a question shares responsibility for the answer.
Use `Assisted-by:` if you want the fact recorded, and keep `Co-authored-by` for people
who can be asked.

That third rule is `gnosis.Actor`'s three-kind split expressed in git. The type
distinguishes `human:`, `agent:` and `check:` because §9.5 needs to refuse a
self-granted approval, and the same distinction is what a commit trailer is for: a
person can be asked why, an agent can be re-run, and a tool can be read. Collapsing them
in the commit log while enforcing them in the corpus would be enforcing a rule the
repository itself does not follow.

## What this does not say

It sets no percentage, no disclosure threshold, and no list of permitted tools. A
threshold would need a rationale under §6.2's own discipline, and there is no measurement
to give one: nobody knows what fraction of a change being model-written predicts a
defect, and inventing a number is what §6.2 exists to prevent.

It also does not ask for a per-file marker. The corpus's own answer to "who wrote this"
is the audit trail and the warrant, both of which record an actor per *decision* rather
than per line — because a decision is the unit somebody can be asked about, and a line
is not.

## Scope

This file governs contributions to this repository. It says nothing about what the
corpus a gnosis bundle holds may contain: that is §5.0.1's question, answered there.
