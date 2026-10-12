// A state's activity is read as a plain member of the state usage: `fill.isActive` is a
// feature chain whose last name the StateActivity import resolves; the parser sees only
// the chain, and the same spelling without the import parses identically.
package F5State {
	private import ScalarValues::*;
	private import StateActivity::*;
	state def Step {
		attribute level1 : Real default 0.0;
		attribute maxLevel : Real default 1.0;
		state fill : Step;
		transition t1 first fill accept when level1 > maxLevel then fill;
		assert constraint c { fill.isActive == true }
		assert constraint d { not fill.isActive }
	}
}
