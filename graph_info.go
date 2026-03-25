package workflow

// GraphTransition represents a directed edge in the workflow's status graph.
type GraphTransition struct {
	From int
	To   int
}

// GraphInfo describes the topology of a workflow's status graph.
type GraphInfo struct {
	Nodes         []int
	StartingNodes []int
	TerminalNodes []int
	Transitions   []GraphTransition
}

// GraphInfo returns the topology of the workflow's status graph.
func (w *Workflow[Type, Status]) GraphInfo() GraphInfo {
	info := w.statusGraph.Info()
	transitions := make([]GraphTransition, len(info.Transitions))
	for i, t := range info.Transitions {
		transitions[i] = GraphTransition{From: t.From, To: t.To}
	}
	return GraphInfo{
		Nodes:         w.statusGraph.Nodes(),
		StartingNodes: info.StartingNodes,
		TerminalNodes: info.TerminalNodes,
		Transitions:   transitions,
	}
}
