package index

import (
	"context"

	"github.com/StevenACoffman/skillet/errs"
)

// Node is one document in the link graph, with its degree in both directions.
//
// Orphan and hub are the two questions the graph is asked, and both are degree
// questions: an orphan has no inbound links, a hub has many. Carrying both counts
// means neither needs a second pass.
type Node struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Title    string `json:"title"`
	Inbound  int    `json:"inbound"`
	Outbound int    `json:"outbound"`
}

// Edge is one resolved link between two documents.
//
// Unresolved links are absent by construction: an edge needs both ends. They are
// not lost — `lint`'s broken-link check reports them as gaps, which is the right
// place, because a gap is a fact about the corpus rather than a shape in the
// graph.
type Edge struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
}

// Graph is the whole link structure.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Orphans returns the nodes nothing links to.
//
// This is the check Luhmann's own practice justifies: a note not connected to the
// network "will get lost in the Zettelkasten, and will be forgotten by the
// Zettelkasten." An orphan is not malformed; it is unreachable.
func (g Graph) Orphans() []Node {
	out := make([]Node, 0)
	for _, n := range g.Nodes {
		if n.Inbound == 0 {
			out = append(out, n)
		}
	}
	return out
}

// Graph reads the whole link structure.
//
// Requires: db is open.
// Ensures: nodes ordered by path and edges by source then target, so two readings
// of one corpus are identical — a graph that reordered itself between runs could
// not be diffed, and diffing is most of what looking at it twice is for. Slices
// are empty, never nil.
func (db *DB) Graph(ctx context.Context) (*Graph, error) {
	const op = "index.DB.Graph"

	g := &Graph{Nodes: make([]Node, 0), Edges: make([]Edge, 0)}

	rows, err := db.sql.QueryContext(ctx, `
		SELECT d.id, d.path, d.title,
		       (SELECT COUNT(*) FROM links WHERE target_document_id = d.id),
		       (SELECT COUNT(*) FROM links WHERE source_document_id = d.id)
		FROM documents d
		ORDER BY d.path`)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &n.Inbound, &n.Outbound); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		g.Nodes = append(g.Nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	edges, err := db.edges(ctx)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	g.Edges = edges
	return g, nil
}

// edges reads the resolved links. Split from Graph because two result sets in one
// function is where the row-scanning boilerplate starts hiding the shape.
func (db *DB) edges(ctx context.Context) ([]Edge, error) {
	const op = "index.DB.edges"

	rows, err := db.sql.QueryContext(ctx, `
		SELECT source_document_id, target_document_id
		FROM links
		WHERE target_document_id IS NOT NULL
		ORDER BY source_document_id, target_document_id`)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]Edge, 0)
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.FromID, &e.ToID); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}
