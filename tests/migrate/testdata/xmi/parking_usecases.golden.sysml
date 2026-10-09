package Context {
    part def Garage;
    part def Ticket;
    part def Driver {
        doc /* The person parking a car. */
    }
    part def Attendant;
    part def Bank;
    part driver : Driver;
    part attendant : Attendant;
    part bank : Bank;
}
package 'Use Cases' {
    use case def 'Park Car' {
        doc /* A driver parks a car in the garage. */
        subject garage : Context::Garage;
        include use case 'pay Fee' : 'Pay Fee';
        /* not migrated: ExtensionPoint 'garage full' — v2 has no extension points; an extending use case is written as a connection to the extended one */
        action def 'Park Flow' {
            first start then Enter;
            action Enter {
                /* body not migrated (the body is empty) */
            }
            first Enter then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        actor driver : Context::Driver;
        actor attendant : Context::Attendant[0..1];
        perform action 'park Flow' : 'Park Flow';
        metadata MigrationMetadata::SynthesizedName about garage, 'pay Fee';
    }
    use case def 'Pay Fee' {
        subject garage : Context::Garage;
        ref part ticket : Context::Ticket;
        include use case authorise : 'Authorise Payment';
        attribute amount : Real;
        actor bank : Context::Bank;
        metadata MigrationMetadata::SynthesizedName about garage, ticket;
    }
    use case def 'Authorise Payment' {
        subject;
        actor bank : Context::Bank;
    }
    use case def 'Wait For Space';
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
        metadata MigrationMetadata::SynthesizedName about 'pay Fee';
    }
    connection def unnamed {
        end 'park Car' : 'Park Car';
        end 'wait For Space' : 'Wait For Space';
        metadata MigrationMetadata::SynthesizedName about 'park Car', 'wait For Space';
    }
    use case 'park Car' : 'Park Car';
    use case 'pay Fee' : 'Pay Fee';
    connection 'park Car to pay Fee' connect 'park Car' to 'pay Fee'; /* «include» */
    use case 'authorise Payment' : 'Authorise Payment';
    connection authorise connect 'pay Fee' to 'authorise Payment'; /* «include» */
    use case 'wait For Space' : 'Wait For Space';
    connection 'wait For Space to park Car' connect 'wait For Space' to 'park Car'; /* extends at extension point(s) 'garage full' when {English} no bay is free */
    metadata MigrationMetadata::SynthesizedName about unnamed;
}
attribute def Real;
connection 'driver to park Car' connect Context::driver to 'Use Cases'::'park Car';
connection 'attendant to park Car' connect Context::attendant to 'Use Cases'::'park Car';
connection settlement : 'Use Cases'::Settlement connect Context::bank to 'Use Cases'::'pay Fee';
