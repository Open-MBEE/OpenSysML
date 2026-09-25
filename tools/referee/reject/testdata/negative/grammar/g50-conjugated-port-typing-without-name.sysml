// Invalid: A conjugated port typing requires a qualified name after ~ (SysML.xtext:974 ConjugatedPortTyping).
package G50ConjugatedPortTypingWithoutName {
    port def P;
    part def D {
        port p : ~;
    }
}
