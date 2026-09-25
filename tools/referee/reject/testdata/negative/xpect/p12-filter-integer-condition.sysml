// Invalid: An element filter condition must have a Boolean result (KerML 8.2.3.2 validateElementFilterMembershipIsBoolean; pilot expressions filter rows).
package P12FilterWithoutBooleanResult {
    filter 1 + 2 * 3;
}
