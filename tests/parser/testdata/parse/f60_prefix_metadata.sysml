package F60PrefixMetadata {
	metadata def M;
	metadata def Classified;
	metadata def service;
	metadata def original;
	metadata def derive;
	port def ServiceDiscovery;
	port def ServiceDiscoveryDD :> ServiceDiscovery;
	requirement def Req1;
	requirement def Req1_1;
	part a;
	part b;

	// Prefix metadata standing where the kind keyword would (SysML.xtext ExtendedUsage).
	#M connect a to b;
	abstract #Classified z;

	part def P {
		#service sd : ServiceDiscovery;
		#service :>> sd : ServiceDiscoveryDD;
	}

	connection def D {
		end #original r1 : Req1;
		end #derive r1_1 : Req1_1;
	}
}
