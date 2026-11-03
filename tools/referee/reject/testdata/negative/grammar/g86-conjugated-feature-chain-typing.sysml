// Invalid: a conjugated port typing is '~' followed by a qualified name; no feature chain follows '~' (SysML.xtext:972 ConjugatedPortTyping, :976 ConjugatedQualifiedName).
package G86ConjugatedFeatureChainTyping {
    port def Link;
    part def Hub {
        port socket : Link;
    }
    part hub : Hub;
    port plug : ~hub.socket;
}
