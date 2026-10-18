package org.eclipse.syson.sysml;

import org.eclipse.emf.ecore.EClass;
import org.eclipse.emf.ecore.EcoreFactory;

public interface SysmlPackage {
    SysmlPackage eINSTANCE = new SysmlPackage() {
        private final EClass namespace = EcoreFactory.eINSTANCE.createEClass();
        {
            namespace.setName("Namespace");
        }

        @Override
        public EClass getNamespace() {
            return namespace;
        }
    };

    EClass getNamespace();
}
