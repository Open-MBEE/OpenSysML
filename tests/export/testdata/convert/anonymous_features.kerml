package AnonymousFeatures {
	classifier Part {
		feature m;
		feature accessedFeature;
		feature engine;
		composite feature carParts;
	}
	feature v : Part {
		feature redefines m {
			feature g;
		}
		feature :>> accessedFeature;
		composite :>> engine = e;
		inout feature redefines accessedFeature redefines m;
	}
	feature e :> Part::engine;
}
