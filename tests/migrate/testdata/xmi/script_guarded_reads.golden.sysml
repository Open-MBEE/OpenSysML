part def Executive {
    attribute tMax : ScalarValues::Real;
    attribute tdiff : ScalarValues::Real default = 0.0;
    action cap {
        first start then idle;
        action idle {
            out z : ScalarValues::Real[1];
            out y : ScalarValues::Real[1];
            rep language "JavaScript" /* tdiff = tdiff * 1;*/
            assign tdiff := tdiff * 1;
        }
        first idle then clamp;
        action clamp {
            in w : ScalarValues::Real[0..1];
            rep language "JavaScript" /* tMax = w;
             * tdiff = tMax * 2;*/
            if w->SequenceFunctions::notEmpty() { assign tMax := w; }
            assign tdiff := tMax * 2;
        }
        first clamp then settle;
        action settle {
            in w : ScalarValues::Real[0..1];
            rep language "JavaScript" /* tMax = 1;
             * tMax = w;
             * tdiff = tMax * 2;*/
            assign tMax := 1;
            if w->SequenceFunctions::notEmpty() { assign tMax := w; }
            assign tdiff := tMax * 2;
        }
        first settle then pick;
        action pick {
            in w : ScalarValues::Real[0..1];
            in v : ScalarValues::Real[0..1];
            rep language "JavaScript" /* tMax = w;
             * tMax = v;
             * tdiff = tMax * 2;*/
            if w->SequenceFunctions::notEmpty() { assign tMax := w; }
            if v->SequenceFunctions::notEmpty() { assign tMax := v; }
            assign tdiff := tMax * 2;
        }
        first pick then early;
        action early {
            in w : ScalarValues::Real[0..1];
            in v : ScalarValues::Real[0..1];
            rep language "JavaScript" /* tMax = w;
             * tdiff = tMax * 2;
             * tMax = v;*/
            if w->SequenceFunctions::notEmpty() { assign tMax := w; }
            assign tdiff := tMax * 2;
            if v->SequenceFunctions::notEmpty() { assign tMax := v; }
        }
        first early then final;
        action final terminate;
        /* flow idle.z to clamp.w not written: the body of 'idle' never assigns idle.z */
        /* flow idle.z to settle.w not written: the body of 'idle' never assigns idle.z */
        /* flow idle.z to pick.w not written: the body of 'idle' never assigns idle.z */
        /* flow idle.y to pick.v not written: the body of 'idle' never assigns idle.y */
        /* flow idle.z to early.w not written: the body of 'idle' never assigns idle.z */
        /* flow idle.y to early.v not written: the body of 'idle' never assigns idle.y */
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
