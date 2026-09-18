package test {

	part def Telescope {
		action def Point;
		action point : Point;
	}

	part def Station {
		part tel : Telescope;

		action def Observe {
			first start then pt;
			perform action pt ::> tel.point;
			perform action again references tel.point;
			perform tel.point;
			first pt then done;
		}
	}
}
