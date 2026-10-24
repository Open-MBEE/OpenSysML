package org.eclipse.syson.sysml.util;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.emf.ecore.EPackage;
import org.eclipse.emf.ecore.util.Switch;
import org.eclipse.syson.sysml.ActionDefinition;
import org.eclipse.syson.sysml.ActionUsage;
import org.eclipse.syson.sysml.CalculationDefinition;
import org.eclipse.syson.sysml.CalculationUsage;
import org.eclipse.syson.sysml.FeatureReferenceExpression;
import org.eclipse.syson.sysml.InvocationExpression;
import org.eclipse.syson.sysml.ResultExpressionMembership;

// Generic like the real switch: a subclass's String-returning case only overrides the real method
// through the bridge javac emits for a generic parent, so the stub must be generic too.
public class SysmlSwitch<T> extends Switch<T> {
    @Override
    protected boolean isSwitchFor(EPackage ePackage) {
        throw new UnsupportedOperationException("stub");
    }

    @Override
    protected T doSwitch(int classifierID, EObject eObject) {
        throw new UnsupportedOperationException("stub");
    }

    public T caseActionDefinition(ActionDefinition object) {
        throw new UnsupportedOperationException("stub");
    }

    public T caseActionUsage(ActionUsage object) {
        throw new UnsupportedOperationException("stub");
    }

    public T caseCalculationDefinition(CalculationDefinition object) {
        throw new UnsupportedOperationException("stub");
    }

    public T caseCalculationUsage(CalculationUsage object) {
        throw new UnsupportedOperationException("stub");
    }

    public T caseFeatureReferenceExpression(FeatureReferenceExpression object) {
        throw new UnsupportedOperationException("stub");
    }

    public T caseInvocationExpression(InvocationExpression object) {
        throw new UnsupportedOperationException("stub");
    }

    public T caseResultExpressionMembership(ResultExpressionMembership object) {
        throw new UnsupportedOperationException("stub");
    }
}
