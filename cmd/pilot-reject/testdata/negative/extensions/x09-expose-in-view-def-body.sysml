// Invalid: Expose is a ViewBodyItem alone; a view def body admits no 'expose' (SysML.xtext:2329 ViewDefinitionBodyItem, :2359 ViewBodyItem).
package X09ExposeInViewDefBody {
    part def Exposed;
    view def V {
        expose Exposed;
    }
}
