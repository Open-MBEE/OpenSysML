package 'Pump System' {
    part def Pump {
        part motor : Motor;
    }
    part def Motor;
    requirement def <R1> 'Guard Fitted' {
        doc /* The pump shall carry a guard. */
        comment /* The pump shall carry a guard. */
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
    viewpoint 'Safety Viewpoint' {
        doc /* What the safety inspector must see. */
        doc /* Show what keeps the pump safe.
         * language: English
         * method: Viewpoints::Safety Viewpoint::Collect Guards
         * presentation: table
         */
        stakeholder 'safety Inspector' : Stakeholders::'Safety Inspector';
        stakeholder maintainer : Stakeholders::Maintainer;
        frame concern {
            doc /* Is every moving part guarded? */
        }
        frame concern {
            doc /* Which hazards does the pump control? */
        }
        action def 'Collect Guards';
    }
    viewpoint 'Operations Viewpoint' {
        doc /* Show how the pump is run. */
        stakeholder operator : Stakeholders::Operator;
    }
    part def 'Plain Block';
}
package Views {
    view 'Safety View' {
        doc /* Everything that keeps the pump safe. */
        satisfy Viewpoints::'Safety Viewpoint';
        view detail :> 'Guard Detail';
        view appendices :> 'Guard Detail'[0..*];
        ref view seeAlso :> 'Operations View';
        expose 'Pump System'::**;
        expose 'Pump System'::'Guard Fitted';
        expose 'Pump System'::Pump::motor;
    }
    view 'Guard Detail';
    view 'Operations View' {
        satisfy Viewpoints::'Operations Viewpoint';
        expose Viewpoints::'Safety Viewpoint';
    }
    view Handbook {
        part def Chapter;
    }
    view 'Odd View';
    /* not migrated: «Expose» Dependency (_exposeDiagram) — the exposed Diagram 'Pump BDD' is notation the tool keeps outside the model, which v2 does not carry; applied stereotypes «Expose» */
    /* not migrated: «Expose» Dependency (_exposeGone) — the exposed element _nowhere is not in the document; applied stereotypes «Expose» */
    /* not migrated: «Expose» Dependency (_exposeExternal) — the exposed Shared.xmi#_sharedBlock lives outside the document and is not written; applied stereotypes «Expose» */
    /* not migrated: «Expose» Dependency (_exposeFromBlock) — the client Pump System::Pump is not a view: v2 exposes elements from a view alone; applied stereotypes «Expose» */
    /* not migrated: «Conform» Dependency (_conformToBlock) — Viewpoints::Plain Block becomes a part def, which a view cannot satisfy; applied stereotypes «Conform» */
    /* not migrated: «Conform» Dependency (_conformFromBlock) — the client Pump System::Pump is not a view: v2 lets a view alone satisfy a viewpoint; applied stereotypes «Conform» */
}
