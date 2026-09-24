// Invalid: Message requires a message declaration with both ends (SysML.xtext:1245 Message).
package G30MessageWithoutTarget {
    part def D {
        event occurrence a;
        event occurrence b;
        message m from a to ;
    }
}
