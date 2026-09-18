// End usages: an interface end may omit its declaration entirely
// (DefaultInterfaceEnd, SysML v2 8.2.2.14.1), while an end in a connection
// definition states one, or omits it after `ref` (ReferenceUsage, 8.2.2.6.3).
package EndUsages {
    port def P;

    interface def Named {
        end source : P;
        end target : P;
    }

    interface def Anonymous {
        end;
        end;
    }

    connection def Attachment {
        end part bracket : Bracket;
        end ref;
    }

    part def Bracket;

    flow def Transfer {
        end ref source;
        end ref target;
    }
}
