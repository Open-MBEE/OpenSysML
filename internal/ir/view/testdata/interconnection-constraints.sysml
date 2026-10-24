package Parametrics {
	private import ScalarValues::Real;

	constraint def Amplify {
		in attribute input : Real;
		in attribute gain : Real;
		out attribute output : Real;
		output == input * gain
	}

	constraint def Mix {
		in attribute a : Real;
		in attribute b : Real;
		out attribute total : Real;
		total == a + b
	}

	// Two constraint usages bound to the owner's attributes and to each other;
	// mix declares a parameter of its own beside those of its def.
	part def Amplifier {
		attribute signal : Real;
		attribute gain : Real;
		attribute noise : Real;
		attribute output : Real;

		constraint amplify : Amplify;
		constraint mix : Mix {
			in attribute weight : Real;
		}

		binding signalIn bind signal = amplify.input;
		binding gainIn bind gain = amplify.gain;
		binding amplified bind amplify.output = mix.a;
		binding noiseIn bind noise = mix.b;
		binding mixed bind mix.total = output;
	}

	// Two usages of one constraint def: each draws the def's parameters as its own.
	part def Stereo {
		attribute left : Real;
		attribute right : Real;
		constraint leftGain : Amplify;
		constraint rightGain : Amplify;
		binding leftIn bind left = leftGain.input;
		binding rightIn bind right = rightGain.input;
	}
}

package ParametricViews {
	private import Views::*;
	private import DiagramLayout::*;

	view amplifierView {
		expose Parametrics::Amplifier;
		render asInterconnectionDiagram;
	}

	// The amplifier's internals as a migrated parametric diagram exposes them:
	// the attributes and constraint usages placed, the def's parameters placed
	// as squares on the usages' borders, and the bindings routed to the squares.
	view placedView {
		expose Parametrics::Amplifier::signal;
		expose Parametrics::Amplifier::gain;
		expose Parametrics::Amplifier::noise;
		expose Parametrics::Amplifier::output;
		expose Parametrics::Amplifier::amplify;
		expose Parametrics::Amplifier::mix;
		expose Parametrics::Amplify::input;
		expose Parametrics::Amplify::gain;
		expose Parametrics::Amplify::output;
		expose Parametrics::Mix::a;
		expose Parametrics::Mix::b;
		expose Parametrics::Mix::total;
		expose Parametrics::Amplifier::mix::weight;
		expose Parametrics::Amplifier::signalIn;
		expose Parametrics::Amplifier::gainIn;
		expose Parametrics::Amplifier::amplified;
		expose Parametrics::Amplifier::noiseIn;
		expose Parametrics::Amplifier::mixed;
		render asInterconnectionDiagram;
		metadata Canvas { width = 900; height = 220; }
		metadata Layout about Parametrics::Amplifier::signal { x = 20; y = 40; width = 100; height = 30; }
		metadata Layout about Parametrics::Amplifier::gain { x = 20; y = 100; width = 100; height = 30; }
		metadata Layout about Parametrics::Amplifier::noise { x = 20; y = 160; width = 100; height = 30; }
		metadata Layout about Parametrics::Amplifier::output { x = 780; y = 40; width = 100; height = 30; }
		metadata Layout about Parametrics::Amplifier::amplify { x = 200; y = 20; width = 200; height = 80; }
		metadata Layout about Parametrics::Amplifier::mix { x = 500; y = 20; width = 200; height = 80; }
		metadata Layout about Parametrics::Amplify::input { x = 200; y = 45; width = 15; height = 15; }
		metadata Layout about Parametrics::Amplify::gain { x = 200; y = 85; width = 15; height = 15; }
		metadata Layout about Parametrics::Amplify::output { x = 385; y = 60; width = 15; height = 15; }
		metadata Layout about Parametrics::Mix::a { x = 500; y = 45; width = 15; height = 15; }
		metadata Layout about Parametrics::Mix::b { x = 500; y = 85; width = 15; height = 15; }
		metadata Layout about Parametrics::Mix::total { x = 685; y = 60; width = 15; height = 15; }
		metadata Layout about Parametrics::Amplifier::mix::weight { x = 590; y = 85; width = 15; height = 15; }
		metadata Route about Parametrics::Amplifier::signalIn { points = (120, 55, 200, 52); }
		metadata Route about Parametrics::Amplifier::gainIn { points = (120, 115, 200, 92); }
		metadata Route about Parametrics::Amplifier::amplified { points = (400, 67, 500, 52); }
		metadata Route about Parametrics::Amplifier::noiseIn { points = (120, 175, 500, 92); }
		metadata Route about Parametrics::Amplifier::mixed { points = (700, 67, 780, 55); }
	}

	// The def's parameter placed once per usage, as the usage's: each square
	// sits where its own Layout says, not where the first usage's does.
	view stereoView {
		expose Parametrics::Stereo::**;
		render asInterconnectionDiagram;
		metadata Layout about Parametrics::Stereo::left { x = 20; y = 40; width = 100; height = 30; }
		metadata Layout about Parametrics::Stereo::right { x = 20; y = 140; width = 100; height = 30; }
		metadata Layout about Parametrics::Stereo::leftGain { x = 200; y = 20; width = 200; height = 60; }
		metadata Layout about Parametrics::Stereo::rightGain { x = 200; y = 120; width = 200; height = 60; }
		metadata Layout about Parametrics::Stereo::leftGain::input { x = 200; y = 30; width = 15; height = 15; }
		metadata Layout about Parametrics::Stereo::rightGain::input { x = 200; y = 160; width = 15; height = 15; }
	}

	// Constraints in a mixed rendering draw as in an interconnection one.
	view mixedView {
		expose Parametrics::Amplifier;
		render OpenSysMLRenderings::asMixedDiagram;
	}
}
