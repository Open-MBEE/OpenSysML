concern 'concern' {
    doc /* Root concern */
    subject;
}
package ViewsModel {
    concern 'concern 2' {
        doc /* Package concern */
        subject;
    }
    concern 'concern' {
        doc /* Shared concern */
        subject;
        stakeholder reviewer : Reviewer;
        metadata MigrationMetadata::SynthesizedName about reviewer;
    }
    part def Reviewer {
        /* «Stakeholder» tags with no v2 form: concern = legacy stakeholder concern */
    }
    view def 'Review Viewpoint' {
        concern 'concern' {
            doc /* Viewpoint concern */
            subject;
        }
        viewpoint 'review Viewpoint' {
            subject;
            stakeholder reviewer : Reviewer;
            frame 'concern 2';
            frame 'concern';
            frame ViewsModel::'concern';
            frame $::'concern';
            frame concern {
                doc /* legacy concern */
            }
            doc /* language: English
             * presentation: dashboard
             */
            require constraint {
                doc /* review purpose */
            }
            metadata MigrationMetadata::SynthesizedName about reviewer;
        }
        satisfy 'review Viewpoint';
        rendering {
            action : createView;
            action : makeView;
            action : 'Tag Method';
        }
        action def createView {
            /* applied stereotype «Create» */
        }
        action 'createView 2' : createView;
        abstract action def makeView {
            /* applied stereotype «Create» */
        }
        action 'makeView 2' : makeView;
        action def 'Tag Method';
        metadata MigrationMetadata::SynthesizedName about 'concern', 'review Viewpoint';
    }
    view def 'Safety Viewpoint' {
        viewpoint 'safety Viewpoint' {
            subject;
            frame 'concern';
        }
        satisfy 'safety Viewpoint';
        metadata MigrationMetadata::SynthesizedName about 'safety Viewpoint';
    }
    enum def Fallback {
        doc /* Enumeration concern */
    }
    view 'Inherited Base' : 'Review Viewpoint';
    view 'Inherited Derived' :> 'Inherited Base' {
        /* conforms to 'Safety Viewpoint' */
    }
    view 'Inherited Same' :> 'Inherited Base';
    view 'Multiple Bases' :> 'Generalized View', 'Tagged View';
    view 'Generalized View' : 'Review Viewpoint';
    view 'Tagged View' : 'Safety Viewpoint';
    view 'Conforming View' : 'Review Viewpoint';
    view 'Combined View' : 'Review Viewpoint' {
        /* conforms to 'Safety Viewpoint' */
    }
    individual view def review :> 'Review Viewpoint';
    metadata MigrationMetadata::SynthesizedName about 'concern 2', 'concern';
}
metadata MigrationMetadata::SynthesizedName about 'concern';
metadata MigrationMetadata::LibraryNameAvoided about ViewsModel { sourceName = "Views"; }
