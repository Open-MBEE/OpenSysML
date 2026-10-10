package runtime

import "slices"

type featureWriteValue struct {
	value        Value
	materialized bool
}

func (ctx *Context) beginFeatureWrite(fv *FeatureValue) func() {
	if ctx.probes > 0 || len(ctx.stateExecutors) == 0 {
		return func() {}
	}
	if ctx.featureWriteDepth == 0 {
		ctx.featureWriteBefore = make(map[*FeatureValue]featureWriteValue)
		ctx.featureWriteOrder = nil
		ctx.featureWriteState = false
	}
	ctx.featureWriteDepth++
	ctx.noteFeatureWrite(fv)
	return func() {
		ctx.endFeatureWrite()
	}
}

// suspendFeatureWrite completes the write under way, so the behaviors it starts write after it.
func (ctx *Context) suspendFeatureWrite() (resume func()) {
	if ctx.featureWriteDepth == 0 {
		return func() {}
	}
	depth := ctx.featureWriteDepth
	ctx.featureWriteDepth = 1
	ctx.endFeatureWrite()
	return func() {
		ctx.featureWriteDepth = depth
		ctx.featureWriteBefore = make(map[*FeatureValue]featureWriteValue)
		ctx.featureWriteOrder = nil
		ctx.featureWriteState = false
	}
}

func (ctx *Context) noteFeatureWrite(fv *FeatureValue) {
	if ctx.probes > 0 || ctx.featureWriteDepth == 0 || fv == nil {
		return
	}
	if _, seen := ctx.featureWriteBefore[fv]; seen {
		return
	}
	ctx.featureWriteBefore[fv] = featureWriteValue{value: fv.HeldValue(), materialized: fv.Materialized}
	ctx.featureWriteOrder = append(ctx.featureWriteOrder, fv)
}

func (ctx *Context) endFeatureWrite() {
	if ctx.featureWriteDepth == 0 {
		return
	}
	ctx.featureWriteDepth--
	if ctx.featureWriteDepth != 0 {
		return
	}
	before, order := ctx.featureWriteBefore, ctx.featureWriteOrder
	stateChanged := ctx.featureWriteState
	ctx.featureWriteBefore, ctx.featureWriteOrder = nil, nil
	ctx.featureWriteState = false
	executors := slices.Clone(ctx.stateExecutors)
	for _, fv := range order {
		prior := before[fv]
		if prior.materialized == fv.Materialized && (!fv.Materialized || heldSame(prior.value, fv.HeldValue())) {
			continue
		}
		for _, exec := range executors {
			exec.observeFeatureWrite(fv)
		}
	}
	if stateChanged {
		for _, exec := range executors {
			exec.observeStateDataWrite()
		}
	}
}

func (ctx *Context) noteStateDataWrite() {
	if ctx.probes > 0 || len(ctx.stateExecutors) == 0 {
		return
	}
	if ctx.featureWriteDepth > 0 {
		ctx.featureWriteState = true
		return
	}
	for _, exec := range slices.Clone(ctx.stateExecutors) {
		exec.observeStateDataWrite()
	}
}

func (ctx *Context) beginChangeRead() func() []*FeatureValue {
	reads := make([]*FeatureValue, 0)
	ctx.readRecorders = append(ctx.readRecorders, reads)
	return func() []*FeatureValue {
		last := len(ctx.readRecorders) - 1
		reads = ctx.readRecorders[last]
		ctx.readRecorders = ctx.readRecorders[:last]
		return reads
	}
}

func (ctx *Context) registerStateExecutor(exec *StateExecutor) {
	for _, registered := range ctx.stateExecutors {
		if registered == exec {
			return
		}
	}
	ctx.stateExecutors = append(ctx.stateExecutors, exec)
}

func (ctx *Context) unregisterStateExecutor(exec *StateExecutor) {
	for i, registered := range ctx.stateExecutors {
		if registered == exec {
			ctx.stateExecutors = append(ctx.stateExecutors[:i], ctx.stateExecutors[i+1:]...)
			return
		}
	}
}
