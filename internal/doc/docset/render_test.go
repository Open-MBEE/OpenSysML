// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package docset_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docset"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A form Render does not render is refused before anything is evaluated, the
// refusal naming every form it does render.
func TestRenderRefusesAnUnknownFormNamingTheForms(t *testing.T) {
	_, err := docset.Render(queryexec.Context{}, nil, nil, docset.Options{Form: "pdf"})
	var unknown *docset.UnknownFormError
	if !errors.As(err, &unknown) || unknown.Form != "pdf" {
		t.Fatalf("Render(pdf) error = %v, want an UnknownFormError for pdf", err)
	}
	if err.Error() != `form "pdf" is not one a document is rendered in: markdown, html or asciidoc` {
		t.Errorf("message = %q", err.Error())
	}
	if docset.FaultOf(err) != docset.FaultArgument {
		t.Errorf("FaultOf(unknown form) = %v, want FaultArgument", docset.FaultOf(err))
	}
}

// Every transport reports a failure under the fault FaultOf charges it to, so
// the classification is the one the service's status codes were written to.
func TestFaultOfClassifiesTheDocumentPipelinesFailures(t *testing.T) {
	origin := symbols.Origin{Doc: "report.sysml"}
	wrapped := &docir.Error{Kind: docir.ErrorQueryExecution, Err: &queryexec.Error{Kind: queryexec.ErrorVisitBudget, Origin: origin}}
	for name, tc := range map[string]struct {
		err   error
		fault docset.Fault
	}{
		"not a document":         {&docset.NotDocumentError{Name: "Demo::Part"}, docset.FaultArgument},
		"an unknown binding":     {&queryexec.Error{Kind: queryexec.ErrorUnknownBinding}, docset.FaultArgument},
		"not a query":            {&queryplan.Error{Kind: queryplan.ErrorNotQueryDefinition}, docset.FaultArgument},
		"a visit budget":         {&queryexec.Error{Kind: queryexec.ErrorVisitBudget}, docset.FaultBudget},
		"a wrapped budget":       {wrapped, docset.FaultBudget},
		"the held-objects bound": {runtime.ErrInstanceLimitExceeded, docset.FaultBudget},
		"a model's query fault":  {&queryexec.Error{Kind: queryexec.ErrorUnknownProperty}, docset.FaultModel},
		"a model's plan fault":   {&docplan.Error{Kind: docplan.ErrorMissingTitle}, docset.FaultModel},
		"no library":             {&docplan.Error{Kind: docplan.ErrorLibraryUnavailable}, docset.FaultInternal},
		"the evaluator's own":    {&docir.Error{Kind: docir.ErrorInvalidContext}, docset.FaultInternal},
		"an unclassified error":  {errors.New("disk full"), docset.FaultInternal},
	} {
		if fault := docset.FaultOf(tc.err); fault != tc.fault {
			t.Errorf("FaultOf(%s) = %v, want %v", name, fault, tc.fault)
		}
	}
	if got := docset.OriginOf(wrapped); got != origin {
		t.Errorf("OriginOf(wrapped) = %v, want the query's origin", got)
	}
	if docset.OriginOf(errors.New("disk full")).Located() {
		t.Error("OriginOf(an unclassified error) is located")
	}
	if docset.QueryOutcomeOf(wrapped) != nil || docset.QueryOutcomeOf(&docplan.Error{}) != nil {
		t.Error("QueryOutcomeOf answers an outcome no query recorded")
	}
}

// A runtime budget spent evaluating a query is a resource fault, wrapped or
// bare; the same unevaluable-feature kind without one remains a model's fault.
func TestFaultOfClassifiesEvaluationBudgets(t *testing.T) {
	for name, tc := range map[string]struct {
		err   error
		fault docset.Fault
	}{
		"steps":             {runtime.ErrStepLimitExceeded, docset.FaultBudget},
		"elements":          {runtime.ErrElementLimitExceeded, docset.FaultBudget},
		"wrapped steps":     {&queryexec.Error{Kind: queryexec.ErrorUnevaluableFeature, Cause: runtime.ErrStepLimitExceeded}, docset.FaultBudget},
		"nested elements":   {&queryexec.Error{Kind: queryexec.ErrorUnevaluableFeature, Cause: fmt.Errorf("evidence: %w", runtime.ErrElementLimitExceeded)}, docset.FaultBudget},
		"a model's failure": {&queryexec.Error{Kind: queryexec.ErrorUnevaluableFeature, Cause: errors.New("bad value")}, docset.FaultModel},
		"an argument":       {&queryexec.Error{Kind: queryexec.ErrorUnknownBinding}, docset.FaultArgument},
	} {
		if fault := docset.FaultOf(tc.err); fault != tc.fault {
			t.Errorf("FaultOf(%s) = %v, want %v", name, fault, tc.fault)
		}
	}
}
