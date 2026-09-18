package AnonymousFeatures {
    classifier def Part {
        feature m;
        feature accessedFeature;
        feature engine;
        composite feature carParts;
    }
    feature v : Part {
        feature redefines m {
            feature g;
        }
        feature redefines accessedFeature;
        composite redefines engine = e;
        inout feature redefines accessedFeature, m;
    }
    feature e subsets Part::engine;
}
