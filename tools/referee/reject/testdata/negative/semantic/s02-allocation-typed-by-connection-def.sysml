// Invalid: An allocation must be typed by allocation definitions (SysML 7.15 Allocations; pilot validateAllocationUsageType).
package S02AllocationTypedByConnectionDef {
    part def P;
    connection def CD {
        end part x : P;
        end part y : P;
    }
    part a : P;
    part b : P;
    allocation al : CD allocate a to b;
}
