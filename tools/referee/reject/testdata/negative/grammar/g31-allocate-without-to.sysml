// Invalid: AllocationUsage requires a to clause naming the allocation target (SysML.xtext:1214 AllocationUsage, :1218 AllocationUsageDeclaration).
package G31AllocateWithoutTo {
    part def D {
        part a;
        part b;
        allocate a ;
    }
}
