// Invalid: a connector's ends must be accessible from its featuring context; a package-level allocation has none, and `ToastBread::applyHeat` and `Toaster::heating` are features of the definitions, not of a usage the allocation can reach (KerML 8.3.4.1 validateConnectorTypeFeaturing; pilot validateConnectorTypeFeaturing).
package S101AllocationEndsThroughDefinition {
    action def ApplyHeat;
    action def ToastBread { action applyHeat : ApplyHeat; }
    part def HeatingSystem;
    part def Toaster {
        part heating : HeatingSystem;
        perform action toastBread : ToastBread;
    }
    allocation heatAllocation allocate ToastBread::applyHeat to Toaster::heating;
}
