package runtime

type probabilityNode struct {
	alternatives    int
	alternativesSet bool
	shares          []float64
	children        map[int]*probabilityNode
	outcome         int
	hasOutcome      bool
}

func (q *exploreQueue) recordProbabilityPath(run *exploreRun, outcome int, leaf bool) {
	node := q.probabilities
	for _, slot := range run.record {
		if !node.alternativesSet || slot.alternatives < node.alternatives {
			node.alternatives = slot.alternatives
			node.alternativesSet = true
		}
		if shares := slotProbabilityShares(slot); shares != nil {
			q.result.weighted = true
			if node.shares == nil {
				node.shares = shares
			}
		}
		child := node.children[slot.taken]
		if child == nil {
			child = &probabilityNode{children: make(map[int]*probabilityNode)}
			node.children[slot.taken] = child
		}
		node = child
	}
	if leaf {
		node.hasOutcome = true
		node.outcome = outcome
	}
}

func slotProbabilityShares(slot exploreSlot) []float64 {
	if slot.kind != slotPick || !slot.described || !slot.choice.Weighted() || slot.taken >= len(slot.choice.Weights) {
		return nil
	}
	total := 0.0
	for _, weight := range slot.choice.Weights {
		total += weight
	}
	if total <= 0 {
		return nil
	}
	shares := make([]float64, len(slot.choice.Weights))
	for i, weight := range slot.choice.Weights {
		shares[i] = weight / total
	}
	return shares
}

type probabilityVectors struct {
	min []float64
	max []float64
}

func setOutcomeProbabilities(root *probabilityNode, outcomes []ExploredOutcome) {
	vectors := root.probabilities(len(outcomes))
	for i := range outcomes {
		outcomes[i].Probability = &ProbabilityRange{Min: vectors.min[i], Max: vectors.max[i]}
	}
}

func (n *probabilityNode) probabilities(count int) probabilityVectors {
	vectors := probabilityVectors{min: make([]float64, count), max: make([]float64, count)}
	if n.hasOutcome {
		vectors.min[n.outcome], vectors.max[n.outcome] = 1, 1
		return vectors
	}
	if len(n.shares) > 0 {
		for alternative, share := range n.shares {
			child := n.children[alternative]
			if child == nil {
				continue
			}
			childVectors := child.probabilities(count)
			for i := range vectors.min {
				vectors.min[i] += share * childVectors.min[i]
				vectors.max[i] += share * childVectors.max[i]
			}
		}
		return vectors
	}
	first := true
	for _, child := range n.children {
		childVectors := child.probabilities(count)
		if first {
			copy(vectors.min, childVectors.min)
			copy(vectors.max, childVectors.max)
			first = false
			continue
		}
		for i := range vectors.min {
			vectors.min[i] = min(vectors.min[i], childVectors.min[i])
			vectors.max[i] = max(vectors.max[i], childVectors.max[i])
		}
	}
	if n.alternativesSet && len(n.children) < n.alternatives {
		clear(vectors.min)
	}
	return vectors
}
