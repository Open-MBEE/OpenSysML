// Invalid: Expose is a ViewBodyItem; a part def body admits no 'expose' (SysML.xtext:2374 Expose, :2359 ViewBodyItem; pilot validateExposeOwningNamespace).
package G67ExposeOutsideViewBody {
    part def Exposed;
    part def System {
        expose Exposed;
    }
}
