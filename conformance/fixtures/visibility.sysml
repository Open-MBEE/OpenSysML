package Vis {
	package Lib {
		part def Pump;
		alias PumpAlias for Pump;
		package Sub {
			part def Valve;
		}
		private part def Secret {
			part inner;
		}
		part held : Secret;
	}
	package Facade {
		public import Lib::Pump;
	}
	package FacadeDeep {
		public import Lib::**;
	}
	package Hidden {
		private import Lib::*;
	}
}
