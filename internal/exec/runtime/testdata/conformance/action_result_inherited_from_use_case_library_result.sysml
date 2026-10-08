package Bat {
	part def Batmobile;
	part def Batman;
	use case def 'Activate rocket booster' {
		subject bm : Batmobile;
		actor driver : Batman;
		objective { doc /* The driver wants to activate the rocket booster. */ }
	}
	action def ActivateRocketBooster :> 'Activate rocket booster' {
		first start;
		then action prepareBoost;
		then action activateBoost;
		then done;
	}
}
