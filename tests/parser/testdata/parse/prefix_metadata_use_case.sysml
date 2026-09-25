package P {
	metadata def <uca> UseCaseAnnotation;
	#uca use case def Withdraw;
	#uca use case u : Withdraw;
	part def System {
		#uca use case nested : Withdraw;
	}
}
