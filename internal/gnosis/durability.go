package gnosis

// The four durability signals of SPEC §14.4, and the support each rests on.
//
// DurabilityNotApplicable is the zero value, and it is the safe one here for a
// reason worth stating, because the obvious reading is the opposite. The failure
// direction this type cannot afford is an unpopulated value claiming a quotation
// *can* be checked offline; "not applicable" claims nothing of the kind. It is also
// literally what the fold returns when it is handed nothing, so a value nobody
// populated and a value folded over no evidence agree — which is the honest
// coincidence rather than a convenient one.
const (
	DurabilityNotApplicable Durability = iota
	DurabilityUnprovable
	DurabilityPartlyProvable
	DurabilityProvable
)

// The three things one piece of a claim's evidence can buy.
//
// SupportNone is the zero value and asserts nothing, the same discipline
// `archive.DispositionUnset` follows one layer down.
//
// This is deliberately **not** `archive.Disposition`. The domain must not learn tier
// 0's record format, and which dispositions are durable is a decision that belongs
// to the package that owns them — `archive.Disposition.Durable()`. The shell
// translates, which is the same route `standards` takes to `archive.Gates`.
const (
	SupportNone Support = iota

	// SupportWeak is a `referenced` source: hash and URI only, no offline proof.
	SupportWeak

	// SupportDurable is an `archived` or `extracted` source, whose quotation
	// validates offline forever.
	SupportDurable
)

// The three weakness classes of SPEC §14.4.1.
//
// WeaknessNotWeak is the zero value: a document nobody classified is not reported,
// which is right because every class here is a *finding* and a zero value must never
// manufacture one.
const (
	WeaknessNotWeak Weakness = iota

	// WeaknessPeripheral is unprovable and not central: informational, and
	// deliberately not listed one by one.
	WeaknessPeripheral

	// WeaknessCitedByProvable is unprovable with provable work resting on it.
	WeaknessCitedByProvable

	// WeaknessLoadBearing is unprovable and at or above the declared in-degree cut.
	WeaknessLoadBearing
)

// Durability is how far a claim, or a document's claims together, can still be
// checked offline (§14.4).
//
// **Orthogonal to Tier, and never composed with it.** A human-reviewed claim resting
// on `referenced` sources is well attested and unprovable; a machine-confirmed claim
// over archived text is weakly attested and fully provable. Neither ordering
// dominates, so gnosis reports both axes and combines neither — a single number over
// the pair would be the score §17 refuses.
type Durability int

// Support is what one archived source buys a claim that cites it.
type Support int

// Weakness is how much an unprovable document matters, per §14.4.1.
//
// The risk is the product of weakness and centrality, so reporting weakness alone
// floods a reader with the peripheral cases that were never a problem. This type is
// the qualifier that stops that, and it is a set of **named states rather than a
// number**, which is §14.4.1's own guard against it becoming a score.
type Weakness int

// String is the phrase §14.4 names each signal by.
func (d Durability) String() string {
	switch d {
	case DurabilityProvable:
		return "provable"
	case DurabilityPartlyProvable:
		return "partly-provable"
	case DurabilityUnprovable:
		return "unprovable"
	case DurabilityNotApplicable:
		return "not-applicable"
	default:
		// An unrecognised value reports the weakest claim it could be making, which
		// is that nothing here is checkable.
		return "not-applicable"
	}
}

// MarshalText renders the signal as a word in the machine envelope, so a reader sees
// §14.4's phrase rather than an integer whose meaning depends on the order above.
func (d Durability) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// String is the class name §14.4.1 uses.
func (w Weakness) String() string {
	switch w {
	case WeaknessLoadBearing:
		return "load-bearing-weak"
	case WeaknessCitedByProvable:
		return "cited-by-provable"
	case WeaknessPeripheral:
		return "peripheral-weak"
	case WeaknessNotWeak:
		return "not-weak"
	default:
		return "not-weak"
	}
}

// MarshalText renders the class as a word in the machine envelope.
func (w Weakness) MarshalText() ([]byte, error) { return []byte(w.String()), nil }

// FoldDurability derives §14.4's signal from what a claim's evidence rests on.
//
// Requires: one Support per archived source the claim cites, in any order.
// Ensures: DurabilityProvable when every resolved source is durable,
// DurabilityUnprovable when none is, DurabilityPartlyProvable when both appear, and
// DurabilityNotApplicable when nothing resolved. Pure, total, order-independent.
//
// **SupportNone counts as neither**, and that is a deliberate silence rather than an
// oversight. An `archive_paths` entry that resolves to no tier-0 record is already
// `archive-closure`'s finding — `archive-unrecorded` — and counting it as weak here
// would report one defect twice under two names, which §10.2.0 refuses for identity
// collision for the same reason.
//
// It takes supports rather than claims, and one function serves both grains: a claim
// folds its own sources, and a document folds every source its claims cite. §14.4's
// table is written at the document grain and its conditions are exactly this fold
// over the union, so computing the two separately would let them disagree about what
// one referenced source does to a page.
func FoldDurability(support []Support) Durability {
	durable, weak := 0, 0
	for _, s := range support {
		switch s {
		case SupportDurable:
			durable++
		case SupportWeak:
			weak++
		case SupportNone:
			// Nothing resolved, so this address says nothing about durability.
		}
	}
	switch {
	case durable == 0 && weak == 0:
		return DurabilityNotApplicable
	case weak == 0:
		return DurabilityProvable
	case durable == 0:
		return DurabilityUnprovable
	default:
		return DurabilityPartlyProvable
	}
}

// ClassifyWeakness reports how much an unprovable document matters (§14.4.1).
//
// Requires: cut is the declared in-degree cut and is positive; inDegree is how many
// documents link here.
// Ensures: WeaknessNotWeak for anything that is not unprovable, so the caller cannot
// classify a provable document by accident. Pure.
//
// **Precedence is by the strength of the reason**, and a document can satisfy two
// conditions at once: central *and* cited by provable work. Load-bearing wins,
// because in-degree is the measure §14.4.1 says the reporting threshold is drawn on;
// a caller holding both facts should say both in its message, which is why they are
// arguments here rather than a private computation.
//
// **The cut is the single boundary, and §14.4.1's "median" is not a second one.**
// That section describes peripheral as *below the corpus median* and load-bearing as
// *at or above a declared cut*, which are two numbers with a gap between them and
// only two treatments — reported, or suppressed. A document in the gap would belong
// to no class, and inventing the median as a second threshold is exactly what §6.2
// prevents. The declared cut is what the corpus can argue about; the median is what
// it should be calibrated to, which `standards/archive.toml` already records as
// pending the corpus's own distribution.
func ClassifyWeakness(d Durability, inDegree, cut int, citedByProvable bool) Weakness {
	if d != DurabilityUnprovable {
		return WeaknessNotWeak
	}
	switch {
	case cut > 0 && inDegree >= cut:
		return WeaknessLoadBearing
	case citedByProvable:
		return WeaknessCitedByProvable
	default:
		return WeaknessPeripheral
	}
}
