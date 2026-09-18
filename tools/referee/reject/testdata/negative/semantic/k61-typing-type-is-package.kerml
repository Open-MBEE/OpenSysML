// Invalid: the type of a keyword-first feature typing must be a type, not a package (KerML 8.3.4.4 FeatureTyping::type is typed Type; pilot fails to link the FeatureTyping — `Couldn't resolve reference to Type 'Q'`).
package K61TypingTypeIsPackage {
    package Q;
    feature f;
    specialization typing f : Q;
}
