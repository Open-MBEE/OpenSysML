package Shadow__V__c_pend0 {
    part marker;
}
package Shadow {
    port def P;
    part def V {
        port q : P;
        port r : P;
        connection c connect q to r;
    }
}
