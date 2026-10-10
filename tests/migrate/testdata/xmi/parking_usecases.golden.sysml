package Context {
    part def Garage;
    part def Ticket;
    part Driver {
        doc /* The person parking a car. */
    }
    part Attendant;
    part Bank;
}
package 'Use Cases' {
    use case 'Park Car' {
        doc /* A driver parks a car in the garage. */
        subject garage : Context::Garage;
        include 'Pay Fee';
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
        perform action 'park Flow' : 'Park Flow';
        metadata MigrationMetadata::SynthesizedName about garage;
    }
    use case 'Pay Fee' {
        subject garage : Context::Garage;
        ref part ticket : Context::Ticket;
        include 'Authorise Payment';
        attribute amount : Real;
        metadata MigrationMetadata::SynthesizedName about garage, ticket;
    }
    use case 'Authorise Payment' {
        subject;
        actor bank :> Context::Bank;
    }
    use case 'Wait For Space';
    use case 'Leave Garage' {
        /* not migrated: Include (_incLost) — the included use case is not in the document; 1 addition reference(s) resolve to nothing in the document (_lostCase) */
        /* not migrated: Extend (_extLost) — the extended use case is not in the document; 1 extendedCase reference(s) resolve to nothing in the document (_lostCase) */
        /* applied stereotype «HyperlinkOwner» */
    }
    use case 'Valet Park' :> 'Park Car';
    part def 'Valet Service' {
        ref use case offer :> 'Valet Park';
    }
    connection def Settlement {
        end bank :> Context::Bank;
        end 'pay Fee' :> 'Pay Fee';
        metadata MigrationMetadata::SynthesizedName about 'pay Fee';
    }
    connection def ParkCarToWaitForSpace {
        end 'park Car' :> 'Park Car';
        end 'wait For Space' :> 'Wait For Space';
        metadata MigrationMetadata::SynthesizedName about 'park Car', 'wait For Space';
    }
    connection 'Park Car to Wait For Space' : ParkCarToWaitForSpace connect 'Park Car' to 'Wait For Space';
    connection 'Park Car includes Pay Fee' connect 'Park Car' to 'Pay Fee';
    connection authorise connect 'Pay Fee' to 'Authorise Payment' {
        doc /* Pay Fee includes Authorise Payment */
    }
    connection 'Wait For Space extends Park Car' connect 'Wait For Space' to 'Park Car' {
        doc /* extends at extension point(s) 'garage full' when {English} no bay is free */
    }
    metadata MigrationMetadata::SynthesizedName about ParkCarToWaitForSpace;
}
attribute def Real;
connection 'Driver to Park Car' connect Context::Driver to 'Use Cases'::'Park Car';
connection 'Attendant to Park Car' connect Context::Attendant to 'Use Cases'::'Park Car';
connection settlement : 'Use Cases'::Settlement connect Context::Bank to 'Use Cases'::'Pay Fee';
