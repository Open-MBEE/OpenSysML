package History {
    state Controller {
        entry; then start;
        state start;
        state outer {
            state first;
            state second;
            history resume;
            deep history resumeDeep;
        }
        state away;

        succession first start then outer.first;

        transition first outer.first when Next then outer.second;
        transition first outer.second when Leave then away;
        transition first away when Back then outer.resume;
    }
}
