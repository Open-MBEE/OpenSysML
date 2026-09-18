// Invalid: AcceptNode requires a payload parameter after accept (SysML.xtext:1442 AcceptNode, :1446 AcceptNodeDeclaration).
package G35AcceptWithoutPayload {
    action def A {
        accept ;
    }
}
