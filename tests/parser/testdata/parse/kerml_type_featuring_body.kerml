package TypeFeaturingBody {
	class Vehicle;
	feature engine;
	feature power;
	featuring engine_by_Vehicle of engine by Vehicle;
	featuring power by engine {
		doc /* The engine of a Vehicle has power. */
		comment /* A relationship body holds annotations. */
	}
}
