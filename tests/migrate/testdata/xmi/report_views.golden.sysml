package 'Pump System' {
    part def Pump {
        part motor : Motor;
    }
    part def Motor;
    requirement def <R1> 'Guard Fitted' {
        doc /* The pump shall carry a guard. */
        comment /* The pump shall carry a guard. */
    }
    view 'Pump BDD' {
        render Views::asTextualNotation;
    }
}
package Stakeholders {
    part def Operator {
        /* «Stakeholder» tags with no v2 form: concern = Keep the pump running. */
    }
    part def 'Safety Inspector';
    part def Maintainer;
}
package Viewpoints {
    view def 'Safety Viewpoint' {
        doc /* What the safety inspector must see. */
        concern 'concern' {
            doc /* Which hazards does the pump control? */
            subject;
        }
        viewpoint 'safety Viewpoint' {
            subject;
            stakeholder 'safety Inspector' : Stakeholders::'Safety Inspector';
            stakeholder maintainer : Stakeholders::Maintainer;
            frame 'concern';
            frame concern {
                doc /* Is every moving part guarded? */
            }
            doc /* language: English
             * presentation: table
             */
            require constraint {
                doc /* Show what keeps the pump safe. */
            }
            metadata MigrationMetadata::SynthesizedName about 'safety Inspector', maintainer;
        }
        satisfy 'safety Viewpoint';
        rendering {
            action : 'Collect Guards';
        }
        action def 'Collect Guards';
        metadata MigrationMetadata::SynthesizedName about 'concern', 'safety Viewpoint';
    }
    view def 'Operations Viewpoint' {
        viewpoint 'operations Viewpoint' {
            subject;
            stakeholder operator : Stakeholders::Operator;
            require constraint {
                doc /* Show how the pump is run. */
            }
            metadata MigrationMetadata::SynthesizedName about operator;
        }
        satisfy 'operations Viewpoint';
        metadata MigrationMetadata::SynthesizedName about 'operations Viewpoint';
    }
    part def 'Plain Block';
}
package ViewsModel {
    view 'Safety View' : Viewpoints::'Safety Viewpoint' {
        doc /* Everything that keeps the pump safe. */
        view detail :> 'Guard Detail';
        view appendices :> 'Guard Detail'[0..*];
        ref view seeAlso :> 'Operations View';
        expose 'Pump System'::**;
        expose 'Pump System'::'Guard Fitted';
        expose 'Pump System'::Pump::motor;
        expose 'Pump System'::'Pump BDD';
    }
    view 'Guard Detail';
    view 'Operations View' : Viewpoints::'Operations Viewpoint' {
        expose Viewpoints::'Safety Viewpoint';
    }
    view Handbook {
        occurrence def Chapter;
    }
    view 'Odd View';
    /* not migrated: «Expose» Dependency (_exposeGone) — the exposed element _nowhere is not in the document; applied stereotypes «Expose» */
    /* not migrated: «Expose» Dependency (_exposeExternal) — the exposed Shared.xmi#_sharedBlock lives outside the document and is not written; applied stereotypes «Expose» */
    /* not migrated: «Expose» Dependency (_exposeFromBlock) — the client Pump System::Pump is not a view: v2 exposes elements from a view alone; applied stereotypes «Expose» */
    /* not migrated: «Conform» Dependency (_conformToBlock) — Viewpoints::Plain Block becomes a part def, which a view cannot satisfy; applied stereotypes «Conform» */
    /* not migrated: «Conform» Dependency (_conformFromBlock) — the client Pump System::Pump is not a view: v2 lets a view alone satisfy a viewpoint; applied stereotypes «Conform» */
}
metadata MigrationMetadata::LibraryNameAvoided about ViewsModel { sourceName = "Views"; }
