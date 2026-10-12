package Library {
    view def 'Recursive Viewpoint' {
        viewpoint 'recursive Viewpoint' {
            subject;
        }
        satisfy 'recursive Viewpoint';
        action def View {
            in ref context : 'Recursive Viewpoint'[1];
            first start then 'recursive call';
            action 'recursive call' : View { in ref :>> context = View::context; }
            first 'recursive call' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        action 'view' : View;
        metadata MigrationMetadata::SynthesizedName about 'recursive Viewpoint';
    }
}
