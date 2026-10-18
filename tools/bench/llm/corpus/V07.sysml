package Metro {
    part def Person;

    part def Station;
    part def Train;
    part def Track;

    part def MetroLine {
        part stations : Station[2..*];
        part trains : Train[1..*];
        part track : Track;
    }

    concern def Punctuality {
        doc /* Trains must run on schedule. */
        subject line : MetroLine;
        stakeholder passenger : Person;
        stakeholder operator : Person;
    }

    concern def Capacity {
        doc /* The line must carry peak-hour demand. */
        subject line : MetroLine;
        stakeholder operator : Person;
    }

    viewpoint def OperationsViewpoint {
        frame concern punctuality : Punctuality;
        frame concern capacity : Capacity;
    }

    view def OperationsView {
        satisfy requirement operationsPerspective : OperationsViewpoint;
    }

    view lineOperations : OperationsView {
        expose MetroLine::**;
    }
}
