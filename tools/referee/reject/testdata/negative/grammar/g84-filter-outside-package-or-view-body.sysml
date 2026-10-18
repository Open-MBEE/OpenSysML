// Invalid: ElementFilterMember is a PackageBodyElement, ViewDefinitionBodyItem and ViewBodyItem; a rendering def body admits no 'filter' (SysML.xtext:227 ElementFilterMember, :2329 ViewDefinitionBodyItem, :2359 ViewBodyItem).
package G84FilterOutsidePackageOrViewBody {
    rendering def R {
        filter true;
    }
}
