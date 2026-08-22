package quarantinecmd

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gate"
)

// join renders a signal list for a person.
//
// It exists because strings.Join does not take a []gate.Signal and the
// alternatives are worse: converting at every call site, or making Signal a bare
// string in the domain so that a typo compiles.
func join(signals []gate.Signal) string {
	names := make([]string, 0, len(signals))
	for _, s := range signals {
		names = append(names, string(s))
	}
	return strings.Join(names, ", ")
}
