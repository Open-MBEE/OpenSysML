connection def WireConnection {
	end supplierPort : Port;
	end consumerPort : Port;
}

connector wire : WireConnection connect a to b;
