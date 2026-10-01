part def Executive {
    attribute tMax : ScalarValues::Real;
    attribute tdiff : ScalarValues::Real default = 0.0;
    action cap {
        first start then idle;
        action idle {
            out z : ScalarValues::Real[1];
            assign tdiff := tdiff * 1;
        }
        first idle then clamp;
        action clamp {
            in w : ScalarValues::Real[0..1];
            if w->SequenceFunctions::notEmpty() { assign tMax := w; }
            assign tdiff := tMax * 2;
        }
        first clamp then settle;
        action settle {
            in w : ScalarValues::Real[0..1];
            assign tMax := 1;
            if w->SequenceFunctions::notEmpty() { assign tMax := w; }
            assign tdiff := tMax * 2;
        }
        first settle then final;
        action final terminate;
        /* flow idle.z to clamp.w not written: the body of 'idle' never assigns idle.z */
        /* flow idle.z to settle.w not written: the body of 'idle' never assigns idle.z */
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
