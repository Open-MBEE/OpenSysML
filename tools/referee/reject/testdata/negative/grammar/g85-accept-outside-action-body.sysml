// Invalid: AcceptNode is an ActionBodyItem; a view body admits no 'accept' (SysML.xtext:1367 ActionBodyItem, :1434 AcceptNode, :2359 ViewBodyItem).
package G85AcceptOutsideActionBody {
    item def S;
    view V {
        accept S;
    }
}
