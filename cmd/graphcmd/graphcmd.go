// Package graphcmd implements the "graph" CLI command.
package graphcmd

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/index"
)

// Config holds the configuration for the graph command.
type Config struct {
	*root.Config
	Orphans bool
	Dot     bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the graph command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("graph").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Orphans, 0, "orphans", "list only documents nothing links to")
	cfg.Flags.BoolVar(&cfg.Dot, 0, "dot", "emit Graphviz DOT instead of a table")
	cfg.Command = &ff.Command{
		Name:      "graph",
		Usage:     "gnosis graph [--orphans] [--dot]",
		ShortHelp: "report the link graph",
		LongHelp: `Report the link structure: every document with its inbound and outbound
degree, and every resolved edge.

--orphans narrows to documents nothing links to. An orphan is not malformed and
is never an error; it is unreachable, which is a different and quieter problem.
A note outside the network, as Luhmann put it, "will get lost in the Zettelkasten,
and will be forgotten by the Zettelkasten."

Unresolved links are absent here, because an edge needs both ends. They are not
lost — ` + "`gnosis lint`" + ` reports them as gaps, which is the right place: a
link to something unwritten is a fact about the corpus rather than a shape in the
graph.

Output is ordered, so two runs over one corpus produce identical text and the
difference between them can be read as a diff.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: open the index, read the graph, render.
func (c *Config) exec(ctx context.Context, _ []string) error {
	db, err := bundle.OpenIndex(ctx, c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	defer func() { _ = db.Close() }()

	graph, err := db.Graph(ctx)
	if err != nil {
		return c.fail(root.ReasonIndexDrift, err)
	}

	if c.Orphans {
		return c.reportOrphans(graph.Orphans())
	}
	return c.report(graph)
}

// reportOrphans renders the narrowed view. An empty result is stated rather than
// left as silence, which would be indistinguishable from not having looked.
func (c *Config) reportOrphans(orphans []index.Node) error {
	if c.JSONL {
		if err := c.EmitOK(map[string]any{"orphans": orphans}); err != nil {
			return fmt.Errorf("graph: %w", err)
		}
		return nil
	}
	for _, n := range orphans {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\n", n.Path, n.Title)
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d orphan(s)\n", len(orphans))
	return nil
}

// report renders the whole graph.
func (c *Config) report(graph *index.Graph) error {
	switch {
	case c.JSONL:
		if err := c.EmitOK(graph); err != nil {
			return fmt.Errorf("graph: %w", err)
		}
	case c.Dot:
		writeDOT(c.Stdout, graph)
	default:
		for _, n := range graph.Nodes {
			_, _ = fmt.Fprintf(c.Stdout, "%d\t%d\t%s\t%s\n",
				n.Inbound, n.Outbound, n.Path, n.Title)
		}
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d document(s), %d resolved edge(s)\n",
		len(graph.Nodes), len(graph.Edges))
	return nil
}

// writeDOT emits Graphviz source.
//
// Nodes are keyed by identifier and labelled by title, which is the same split
// the corpus itself makes: the identifier is what is stable, the title is what a
// person reads. Labels are quoted with strconv.Quote because a title is arbitrary
// prose and an unescaped quotation mark in it would produce a DOT file that does
// not parse.
func writeDOT(w io.Writer, graph *index.Graph) {
	_, _ = fmt.Fprintln(w, "digraph gnosis {")
	_, _ = fmt.Fprintln(w, "  rankdir=LR;")
	for _, n := range graph.Nodes {
		label := n.Title
		if label == "" {
			label = n.Path
		}
		_, _ = fmt.Fprintf(w, "  %s [label=%s];\n",
			strconv.Quote(n.ID), strconv.Quote(label))
	}
	for _, e := range graph.Edges {
		_, _ = fmt.Fprintf(w, "  %s -> %s;\n",
			strconv.Quote(e.FromID), strconv.Quote(e.ToID))
	}
	_, _ = fmt.Fprintln(w, "}")
}

// fail adapts root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("graph: %w", c.Fail(reason, cause))
}
