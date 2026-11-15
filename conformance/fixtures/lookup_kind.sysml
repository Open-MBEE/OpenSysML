package Col__V__c_pend0 { part marker; }
package Col {
    port def P;
    part def V { port q : P; port r : P; connection c connect q to r; }
    part def W { part a; part b; part { part inner; } part '@2' { part named; } }
}
