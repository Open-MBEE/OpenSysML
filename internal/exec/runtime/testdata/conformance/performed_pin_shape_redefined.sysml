// A pin bound through redefinition takes the effective multiplicity of what it redefines:
// `in :>> value = 5` over a [1] pin reads bare, over a bare pin reads as a one-element
// sequence; a same-position parameter of a specializing action def implicitly redefines
// the general one and inherits its multiplicity unless it states its own, so `in again`
// over a [1] pin reads bare and `in again : Integer[1]` over a bare pin reads bare too.
package test {
    private import ScalarValues::*;

    action def Scalar {
        in value : Integer[1];
        out echo : Integer[1];
        first start;
        then action e { assign echo := value; }
        then done;
    }

    action def Loose {
        in value : Integer;
        out echo : Integer;
        first start;
        then action e { assign echo := value; }
        then done;
    }

    action def ScalarAgain :> Scalar {
        in again : Integer;
    }

    action def LooseNarrowed :> Loose {
        in again : Integer[1];
    }

    action probe {
        out fromScalar : Integer[1];
        out fromLoose : Integer;
        out fromImplicit : Integer[1];
        out implicitAgain : Integer;
        out narrowedAgain : Integer[1];
        out narrowedEcho : Integer;
        first start;
        then action s : Scalar { in :>> value = 5; }
        then action l : Loose { in :>> value = 5; }
        then action a : ScalarAgain { in :>> again = 7; }
        then action n : LooseNarrowed { in :>> again = 8; }
        then action read {
            assign fromScalar := s.echo;
            assign fromLoose := l.echo;
            assign fromImplicit := a.echo;
            assign implicitAgain := a.again;
            assign narrowedAgain := n.again;
            assign narrowedEcho := n.echo;
        }
        then done;
    }
}
