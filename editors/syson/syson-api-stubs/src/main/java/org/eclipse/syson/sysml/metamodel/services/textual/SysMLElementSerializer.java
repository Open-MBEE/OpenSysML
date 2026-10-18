package org.eclipse.syson.sysml.metamodel.services.textual;

import java.util.Objects;
import java.util.function.Consumer;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.syson.sysml.ActionDefinition;
import org.eclipse.syson.sysml.ActionUsage;
import org.eclipse.syson.sysml.CalculationDefinition;
import org.eclipse.syson.sysml.FeatureReferenceExpression;
import org.eclipse.syson.sysml.InvocationExpression;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;
import org.eclipse.syson.sysml.util.SysmlSwitch;

// Mirrors which cases SysON overrides with a String return; the others stay generic on SysmlSwitch.
public class SysMLElementSerializer extends SysmlSwitch<String> {
    public SysMLElementSerializer(SysMLSerializingOptions options, Consumer<Status> reportConsumer) {
        Objects.requireNonNull(options);
        Objects.requireNonNull(reportConsumer);
    }

    @Override
    public String doSwitch(EObject eObject) {
        throw new UnsupportedOperationException("stub");
    }

    @Override
    public String caseActionDefinition(ActionDefinition actionDefinition) {
        throw new UnsupportedOperationException("stub");
    }

    @Override
    public String caseActionUsage(ActionUsage actionUsage) {
        throw new UnsupportedOperationException("stub");
    }

    @Override
    public String caseCalculationDefinition(CalculationDefinition calculationDefinition) {
        throw new UnsupportedOperationException("stub");
    }

    @Override
    public String caseFeatureReferenceExpression(FeatureReferenceExpression expression) {
        throw new UnsupportedOperationException("stub");
    }

    @Override
    public String caseInvocationExpression(InvocationExpression expression) {
        throw new UnsupportedOperationException("stub");
    }
}
