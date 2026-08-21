// Package command holds the write protocol as values (SPEC §4.6.2).
//
// Reads bypass the write coordinator by requirement: `lint`, `search`, `show`,
// and `graph` open the index directly and MUST work with nothing serving, because
// a corpus has to be inspectable when no daemon is running. Writes go through the
// coordinator, so the coordinator is a command bus and what crosses it is a
// command.
//
// A command is a **value**, not a verb: one type per write operation, carrying
// everything needed to decide whether and how to execute. Three properties follow,
// and they are the reason this is a type rather than a function signature.
//
// Every transport inherits the gating fields for free. The CLI populates them from
// flags, a socket or HTTP or MCP transport from a payload, an internal caller
// directly — and none of them can construct the command without them.
// Review-gating is therefore a property of the type rather than of the wire
// format, which is stronger than a schema-validated protocol field because it also
// binds the callers a protocol never sees.
//
// §9.4's diff guarantee becomes constructible rather than promised. That section
// requires the diff the gate approved to be the diff that lands. If preview and
// apply were two commands or two code paths, the guarantee would reduce to a claim
// that two functions agree — the kind of claim this design refuses elsewhere. As
// one command differing in one field they cannot diverge.
//
// Validation belongs to the command, not to the coordinator. A gating rule
// enforced by the executor is a rule every future executor has to remember; a
// gating rule enforced by Validate is one no caller can route around.
//
// Everything here is pure. Nothing in this package touches a disk, a database, or
// a clock.
package command

// A compile-time assertion that Promote is a Command. Without it the interface is
// satisfied by accident, and a renamed method would surface at the coordinator
// rather than here. It also keeps `unused` from deleting an interface that has no
// caller yet — which it did once, silently, before this line existed.
var _ Command = (*Promote)(nil)

// Command is one write, as a value.
//
// The interface carries Validate rather than leaving it to the coordinator so
// that "no transport can skip validation" is checkable: a transport that
// deserialises into a Command still has to hand it to something that calls
// Validate, and the coordinator's contract says it does so first.
type Command interface {
	// Op names the operation, for a message and an audit row. It is a method
	// rather than a field so a command cannot misreport what it is.
	Op() string

	// Effect reports whether this command writes.
	Effect() Effect

	// Validate reports why this command is not executable as constructed, or nil.
	Validate() error
}
