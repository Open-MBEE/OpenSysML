package Context {
    part def Garage;
    part def Ticket;
    part def Driver {
        doc /* The person parking a car. */
    }
    part def Attendant;
    part def Bank;
}
package 'Use Cases' {
    use case def 'Park Car' {
        doc /* A driver parks a car in the garage. */
        subject garage : Context::Garage;
        include use case 'pay Fee' : 'Pay Fee';
        /* not migrated: ExtensionPoint 'garage full' — v2 has no extension points; an extending use case is written as a dependency on the extended one */
        action def 'Park Flow' {
            first start then Enter;
            action Enter {
                /* body not migrated (the body is empty) */
            }
            first Enter then final;
            action final terminate;
        }
        actor driver : Context::Driver;
        actor attendant : Context::Attendant[0..1];
        perform action 'park Flow' : 'Park Flow';
    }
    use case def 'Pay Fee' {
        subject garage : Context::Garage;
        ref part ticket : Context::Ticket;
        include use case authorise : 'Authorise Payment';
        attribute amount : Real;
        actor bank : Context::Bank;
    }
    use case def 'Authorise Payment' {
        subject;
        actor bank : Context::Bank;
    }
    use case def 'Wait For Space' {
        dependency 'Wait For Space' to 'Park Car'; /* extends at extension point(s) 'garage full' when {English} no bay is free */
    }
    use case def 'Leave Garage' {
        /* not migrated: Include (_incLost) — the included use case is not in the document; 1 addition reference(s) resolve to nothing in the document (_lostCase) */
        /* not migrated: Extend (_extLost) — the extended use case is not in the document; 1 extendedCase reference(s) resolve to nothing in the document (_lostCase) */
        /* applied stereotype «HyperlinkOwner» */
    }
    use case def 'Valet Park' :> 'Park Car';
    part def 'Valet Service' {
        ref use case offer : 'Valet Park';
    }
    connection def Settlement {
        end bank : Context::Bank;
        end 'pay Fee' : 'Pay Fee';
    }
    connection def unnamed {
        end 'park Car' : 'Park Car';
        end 'wait For Space' : 'Wait For Space';
    }
}
attribute def Real;
