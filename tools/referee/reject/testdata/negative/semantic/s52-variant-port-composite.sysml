// Invalid: A port usage whose owning type is not a port definition or port usage must be referential, and a variant has no owning type (SysML 7.12 Ports; pilot validatePortUsageIsReference).
package S52VariantPortComposite {
    port def PD;
    port def Q {
        variation port vp : PD {
            variant port a : PD;
        }
    }
    part holder {
        port q : Q {
            variation port vq : ~PD {
                variant port b : ~PD;
            }
        }
    }
}
