package Lib {
	public package Widgets {
		public part def Widget;
	}

	private package Gadgets {
		part def Gadget;
	}
}

package App {
	private import Lib::*;
	alias W for Lib::Widgets;
	dependency from App to Lib;
}
