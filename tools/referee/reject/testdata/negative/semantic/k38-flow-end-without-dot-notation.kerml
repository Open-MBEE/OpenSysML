// Invalid: a flow end must subset an identifiable feature through dot notation (KerML 8.3.4.9 validateFlowEndSubsetting; pilot validateFlowEndSubsetting).
package K38FlowEndWithoutDotNotation {
    class T;
    class C {
        feature x : T;
        feature y : T;
        flow from x to y;
    }
}
