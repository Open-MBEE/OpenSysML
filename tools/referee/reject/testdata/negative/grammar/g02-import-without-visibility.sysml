// Invalid: ImportPrefix requires an explicit visibility keyword before 'import' (SysML.xtext ImportPrefix).
package G02ImportWithoutVisibility {
    package Q {
        part def D;
    }
    import Q::*;
}
