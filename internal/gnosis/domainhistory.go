package gnosis

import (
	"sort"
	"strings"
)

// Adjudicated is one recorded decision, as the domain-history fold needs to see it.
//
// Narrower than a warrant on purpose: the fold asks who decided something under a
// subject, and a type carrying the rationale would invite a caller to weigh the reasoning
// — which is the review queue's job for a person and never a count's.
type Adjudicated struct {
	// By is the adjudicator, as OKF §7's grammar writes it. A raw string for
	// `Warrant.By`'s reason: §14.1.1 makes frontmatter actors a wider population than
	// `Actor`, and rejecting a conformant document for the shape of an optional family
	// is what OKF §11 forbids.
	By string

	// Subject is the resolved subject key the decided claim was about, or empty for a
	// claim declaring none.
	Subject SubjectKey
}

// DomainCount is how many claims one person has previously adjudicated under a subject
// prefix.
type DomainCount struct {
	By    string
	Count int
}

// FoldDomainHistory is §10.6.2's display, computed rather than configured.
//
// Requires: decided are the corpus's recorded adjudications; subject is the key under
// adjudication now.
// Ensures: one entry per adjudicator with at least one prior decision under the
// subject's dotted prefix, ordered by count descending then by actor, so two runs over
// one corpus present the same list. Pure.
//
// # Why this is shown and never enforced
//
// §10.6.2 refuses to require a domain-matched co-signer, and gives two reasons that both
// have teeth. A declared capability roster "is a political artifact": somebody must write
// down, about colleagues, who is qualified, then maintain it as people change focus — and
// it hard-blocks whenever the sole holder is unavailable. A capability *derived from
// behaviour* is worse: "you may adjudicate `db.*` because you have adjudicated `db.*`"
// entrenches whoever arrived first, is gameable by adjudicating easy claims to acquire
// standing over hard ones, and mistakes activity for competence.
//
// So the count is displayed beside the decision and grants nothing. **It cannot be gamed
// into authority because there is nothing to acquire** — which is the property that makes
// showing it safe, and the reason this returns a list a queue renders rather than a
// predicate a gate could call.
//
// # The prefix, and why it is dotted
//
// §10.6.2's own example counts "prior adjudications under `retry.*`" for a conflict on
// `retry.max_attempts`, so the population is the subject's first dotted segment. A person
// who has decided four things about retries has context on a fifth; one who has decided
// four things about database isolation does not, and counting every adjudication in the
// corpus would report seniority rather than domain.
func FoldDomainHistory(decided []Adjudicated, subject SubjectKey) []DomainCount {
	prefix := DomainOf(subject)
	if prefix == "" {
		return nil
	}
	counts := map[string]int{}
	for _, d := range decided {
		if strings.TrimSpace(d.By) == "" || DomainOf(d.Subject) != prefix {
			continue
		}
		counts[d.By]++
	}

	out := make([]DomainCount, 0, len(counts))
	for by, count := range counts {
		out = append(out, DomainCount{By: by, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		// By actor within a count, because map iteration is randomized and a queue
		// whose two equally-experienced colleagues swapped places between loads is
		// one a reviewer cannot trust to be saying anything.
		return out[i].By < out[j].By
	})
	return out
}

// DomainOf is the subject's domain: everything before the first dot, or the whole key.
//
// Requires: nothing.
// Ensures: the empty string for an empty key, so a claim with no subject has no domain
// and matches nothing. Pure.
//
// A key with no dot is its own domain, which is the reading that does not surprise: a
// corpus whose subjects are flat words gets per-subject counts rather than one bucket
// holding everything.
func DomainOf(subject SubjectKey) SubjectKey {
	head, _, found := strings.Cut(string(subject), ".")
	if !found {
		return subject
	}
	return SubjectKey(head)
}
