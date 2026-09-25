connection def PressureSeat {
	end [1] part bead : TireBead;
	end [1] mountingRim : TireMountingRim;
}

connection def Anonymous {
	end [1] : TireBead;
	end [1] part named : TireMountingRim;
}

connection def Crossing {
	end in x1 : TireBead [1] item x : TireBead;
	end derived ref x2 [1] item y : TireMountingRim;
}

connection def CrossingAfterMultiplicity {
	end [1] :> beads item x : TireBead;
	end [0..1] subsets rims : TireMountingRim item y : TireMountingRim;
}

connection def OrderedCrossing {
	end beads [1..*] ordered item x : TireBead;
	end [*] nonunique ordered :> rims item y : TireMountingRim;
	end ordered item z : TireBead;
	end [2] nonunique ref w : TireBead;
}

interface def FuelInterface {
	end supplierPort : FuelOutPort;
	end part consumerPort;
}

part wheelAssy {
	connection : PressureSeat connect
		bead references t.bead to
		mountingRim references w.rim;
	interface : FuelInterface connect
		supplierPort ::> tankAssy.fuelTankPort to
		consumerPort ::> eng.engineFuelPort;
}
