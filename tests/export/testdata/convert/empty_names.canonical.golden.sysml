package 'Empty Names' {
    part def Pt {
        part x;
    }
    part '' : Pt;
    part '   ' : Pt;
    part : Pt;
    part r subsets '';
    part s subsets ''.x;
    package Inner {
        package '' {
            part def Q;
        }
        part q : ''::Q;
    }
    alias Al for Inner::'';
}
package Consumer {
    private import 'Empty Names'::*;
    part t subsets '';
}
package Shorts {
    part def <''> Short;
    part u : Short;
}
