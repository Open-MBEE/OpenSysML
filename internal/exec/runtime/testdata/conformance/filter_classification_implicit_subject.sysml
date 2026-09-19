package test {
	metadata def Safety;

	// A classification that leaves its subject out classifies the object being
	// evaluated, so it is judged by what the object was instantiated from.
	#Safety part def Vehicle {
		assert constraint tagged { @Safety }
		assert constraint namedSelf { self @ Safety }
		assert constraint notComfort { not (@Comfort) }
	}
	metadata def Comfort;
}
