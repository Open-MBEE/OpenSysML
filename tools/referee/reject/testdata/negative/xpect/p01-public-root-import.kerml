// Invalid: a root-namespace import must be private (KerML 8.2.3.5.2 validateImportTopLevelVisibility; pilot Import_RootNamspace_invalid.kerml.xt).
public import Base::Anything;
package P01PublicRootImport {
    class C;
}
