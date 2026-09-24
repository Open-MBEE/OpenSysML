// Invalid: FramedConcernMember requires a concern usage after the frame keyword (SysML.xtext:2073 FramedConcernMember).
package G19FrameWithoutConcern {
    requirement def R {
        frame ;
    }
}
