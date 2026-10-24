// Invalid: TargetSuccessionMember is an ActionBodyItem; a view body admits no 'then <target>;' succession (SysML.xtext:1367 ActionBodyItem, :1693 TargetSuccessionMember, :2359 ViewBodyItem).
package X11TargetSuccessionOutsideActionBody {
    view V {
        action x;
        then y;
    }
}
