// Behaviour views positioned the way a migrated Cameo diagram is: every symbol
// has its bounds, every edge its route, and the unnamed nodes carry names the
// migration made up.
package Notation {
	private import ScalarValues::*;

	item def Frame;
	attribute def Done;
	attribute def Abort;

	action def Sense {
		in mask : String;
		in guide : Boolean;
		out image : Frame;
	}

	action def Process {
		in image : Frame;
	}

	action def Report;

	action def Acquire {
		attribute i : Integer;

		action pick {
			out result : String = "SH-0";
		}
		action flag {
			out result : Boolean = true;
		}
		action sense : Sense;
		action process : Process;
		action report : Report;
		action step {
			assign i := i + 1;
		}
		fork split;
		join gather;
		decide check;
		merge again;
		action final terminate;

		succession 'start to split' first start then split;
		succession 'split to sense' first split then sense;
		succession 'split to report' first split then report;
		succession 'sense to process' first sense then process;
		succession 'process to gather' first process then gather;
		succession 'report to gather' first report then gather;
		succession 'gather to again' first gather then again;
		succession 'again to check' first again then check;
		succession 'check to step' first check if i < 3 then step;
		succession 'check to final' first check if i >= 3 then final;
		succession 'step to again' first step then again;

		flow 'pick.result to sense.mask' from pick.result to sense.mask;
		flow 'flag.result to sense.guide' from flag.result to sense.guide;
		flow 'sense.image to process.image' from sense.image to process.image;

		metadata MigrationMetadata::SynthesizedName about pick, flag, step, split, gather, check, again, final,
			'start to split', 'split to sense', 'split to report', 'sense to process', 'process to gather', 'report to gather',
			'gather to again', 'again to check', 'check to step', 'check to final', 'step to again',
			'pick.result to sense.mask', 'flag.result to sense.guide', 'sense.image to process.image';
	}

	action def Initialize;
	action def Align;
	action def Settle;

	state def Alignment {
		attribute timer : Integer;

		transition 'start then Initializing' first start then Initializing;
		state Initializing {
			do action : Initialize;
		}
		state Aligning {
			entry action prime { assign timer := 0; }
			do action align : Align;
			exit action : Settle;
		}
		state Phasing {
			entry action : Initialize;
			transition 'start then Coarse' first start then Coarse;
			state Coarse {
				do action : Align;
			}
			state Fine {
				do action : Align;
			}
			state Tracking parallel {
				state Guiding { state guide; }
				state Focusing { state focus; }
			}
			choice which;
			junction settle;
			fork spread;
			join collect;
			shallow history resume;
			deep history recall;
			transition 'Coarse to which' first Coarse accept Done then which;
			transition 'which to Fine' first which if timer > 0 then Fine;
			transition 'which to settle' first which then settle;
			transition 'settle to spread' first settle then spread;
			transition 'spread to guide' first spread then Tracking.Guiding.guide;
			transition 'spread to focus' first spread then Tracking.Focusing.focus;
			transition 'guide to collect' first Tracking.Guiding.guide then collect;
			transition 'focus to collect' first Tracking.Focusing.focus then collect;
			transition 'collect to Fine' first collect then Fine;
			transition 'resume to Coarse' first resume then Coarse;
			transition 'recall to Fine' first recall then Fine;
			metadata MigrationMetadata::SynthesizedName about 'start then Coarse', 'Coarse to which', 'which to Fine', 'which to settle',
				'settle to spread', 'spread to guide', 'spread to focus', 'guide to collect', 'focus to collect', 'collect to Fine', 'resume to Coarse', 'recall to Fine';
		}
		transition 'Initializing to Aligning' first Initializing accept Done then Aligning;
		transition 'Aligning to done' first Aligning accept Done if timer > 0 then done;
		transition 'Aligning to Initializing' first Aligning accept Abort do action retry : Initialize then Initializing;
		transition 'Aligning to Phasing' first Aligning accept Phase then Phasing;
		transition 'Phasing to Aligning' first Phasing accept Abort then Aligning;
		transition 'Aligning to resume' first Aligning accept Resume then Phasing.resume;
		transition 'Initializing to recall' first Initializing accept Recall then Phasing.recall;
		metadata MigrationMetadata::SynthesizedName about 'start then Initializing', 'Initializing to Aligning', 'Aligning to done', 'Aligning to Initializing',
			'Aligning to Phasing', 'Phasing to Aligning', 'Aligning to resume', 'Initializing to recall';
	}
}

package NotationViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view acquireView : ActionFlowView {
		expose Notation::Acquire;
		@Canvas { width = 700; height = 560; }
		metadata Layout about Notation::Acquire { x = 0; y = 0; width = 700; height = 560; }
		metadata Layout about Notation::Acquire::pick { x = 20; y = 20; width = 80; height = 40; }
		metadata Layout about Notation::Acquire::flag { x = 140; y = 20; width = 80; height = 40; }
		metadata Layout about Notation::Acquire::start { x = 330; y = 30; width = 15; height = 15; }
		metadata Layout about Notation::Acquire::split { x = 150; y = 100; width = 200; height = 10; }
		metadata Layout about Notation::Acquire::sense { x = 60; y = 160; width = 160; height = 60; }
		metadata Layout about Notation::Acquire::report { x = 300; y = 160; width = 140; height = 60; }
		metadata Layout about Notation::Acquire::process { x = 60; y = 280; width = 160; height = 60; }
		metadata Layout about Notation::Acquire::gather { x = 150; y = 380; width = 200; height = 10; }
		metadata Layout about Notation::Acquire::again { x = 240; y = 430; width = 20; height = 20; }
		metadata Layout about Notation::Acquire::check { x = 240; y = 480; width = 20; height = 20; }
		metadata Layout about Notation::Acquire::step { x = 400; y = 440; width = 100; height = 40; }
		metadata Layout about Notation::Acquire::final { x = 240; y = 530; width = 20; height = 20; }
		metadata Route about Notation::Acquire::'start to split' { points = (337, 45, 337, 100); }
		metadata Route about Notation::Acquire::'split to sense' { points = (140, 110, 140, 160); }
		metadata Route about Notation::Acquire::'split to report' { points = (370, 110, 370, 160); }
		metadata Route about Notation::Acquire::'sense to process' { points = (100, 220, 100, 280); }
		metadata Route about Notation::Acquire::'process to gather' { points = (140, 340, 140, 380); }
		metadata Route about Notation::Acquire::'report to gather' { points = (370, 220, 370, 380); }
		metadata Route about Notation::Acquire::'gather to again' { points = (250, 390, 250, 430); }
		metadata Route about Notation::Acquire::'again to check' { points = (250, 450, 250, 480); }
		metadata Route about Notation::Acquire::'check to step' { points = (260, 490, 400, 490); }
		metadata Route about Notation::Acquire::'check to final' { points = (250, 500, 250, 530); }
		metadata Route about Notation::Acquire::'step to again' { points = (450, 440, 450, 440, 260, 440); }
		metadata Route about Notation::Acquire::'pick.result to sense.mask' { points = (60, 66, 60, 130, 90, 130, 90, 160); }
		metadata Route about Notation::Acquire::'flag.result to sense.guide' { points = (180, 66, 180, 160); }
		metadata Route about Notation::Acquire::'sense.image to process.image' { points = (180, 220, 180, 280); }
		metadata Note about Notation::Acquire::sense { text = "Acquire a new star"; x = 480; y = 240; width = 140; height = 50; }
	}

	view alignmentView : StateTransitionView {
		expose Notation::Alignment;
		@Canvas { width = 1000; height = 400; }
		metadata Layout about Notation::Alignment { x = 0; y = 0; width = 1000; height = 400; }
		metadata Layout about Notation::Alignment::start { x = 40; y = 30; width = 15; height = 15; }
		metadata Layout about Notation::Alignment::Initializing { x = 20; y = 100; width = 140; height = 70; }
		metadata Layout about Notation::Alignment::Aligning { x = 260; y = 100; width = 200; height = 110; }
		metadata Layout about Notation::Alignment::done { x = 350; y = 320; width = 20; height = 20; }
		metadata Layout about Notation::Alignment::Phasing { x = 520; y = 20; width = 460; height = 360; }
		metadata Layout about Notation::Alignment::Phasing::start { x = 560; y = 60; width = 15; height = 15; }
		metadata Layout about Notation::Alignment::Phasing::Coarse { x = 540; y = 120; width = 120; height = 60; }
		metadata Layout about Notation::Alignment::Phasing::which { x = 590; y = 220; width = 20; height = 20; }
		metadata Layout about Notation::Alignment::Phasing::Fine { x = 700; y = 200; width = 120; height = 60; }
		metadata Layout about Notation::Alignment::Phasing::settle { x = 592; y = 290; width = 16; height = 16; }
		metadata Layout about Notation::Alignment::Phasing::spread { x = 700; y = 293; width = 60; height = 10; }
		metadata Layout about Notation::Alignment::Phasing::Tracking { x = 790; y = 270; width = 120; height = 100; }
		metadata Layout about Notation::Alignment::Phasing::Tracking::Guiding { x = 790; y = 290; width = 120; height = 40; }
		metadata Layout about Notation::Alignment::Phasing::Tracking::Guiding::guide { x = 810; y = 295; width = 80; height = 30; }
		metadata Layout about Notation::Alignment::Phasing::Tracking::Focusing { x = 790; y = 330; width = 120; height = 40; }
		metadata Layout about Notation::Alignment::Phasing::Tracking::Focusing::focus { x = 810; y = 335; width = 80; height = 30; }
		metadata Layout about Notation::Alignment::Phasing::collect { x = 930; y = 250; width = 10; height = 120; }
		metadata Layout about Notation::Alignment::Phasing::resume { x = 860; y = 80; width = 20; height = 20; }
		metadata Layout about Notation::Alignment::Phasing::recall { x = 920; y = 80; width = 20; height = 20; }
		metadata Route about Notation::Alignment::'start then Initializing' { points = (47, 45, 47, 100); }
		metadata Route about Notation::Alignment::'Initializing to Aligning' { points = (160, 130, 260, 130); }
		metadata Route about Notation::Alignment::'Aligning to done' { points = (360, 210, 360, 320); }
		metadata Route about Notation::Alignment::'Aligning to Initializing' { points = (260, 180, 90, 180, 90, 170); }
		metadata Route about Notation::Alignment::'Aligning to Phasing' { points = (460, 130, 520, 130); }
		metadata Route about Notation::Alignment::'Phasing to Aligning' { points = (520, 180, 460, 180); }
		metadata Route about Notation::Alignment::'Aligning to resume' { points = (460, 110, 490, 110, 490, 10, 870, 10, 870, 80); }
		metadata Route about Notation::Alignment::'Initializing to recall' { points = (160, 110, 200, 110, 200, 5, 930, 5, 930, 80); }
		metadata Route about Notation::Alignment::Phasing::'start then Coarse' { points = (567, 75, 567, 120); }
		metadata Route about Notation::Alignment::Phasing::'Coarse to which' { points = (600, 180, 600, 220); }
		metadata Route about Notation::Alignment::Phasing::'which to Fine' { points = (610, 230, 700, 230); }
		metadata Route about Notation::Alignment::Phasing::'which to settle' { points = (600, 240, 600, 290); }
		metadata Route about Notation::Alignment::Phasing::'settle to spread' { points = (608, 298, 700, 298); }
		metadata Route about Notation::Alignment::Phasing::'spread to guide' { points = (760, 298, 785, 298, 785, 310, 810, 310); }
		metadata Route about Notation::Alignment::Phasing::'spread to focus' { points = (760, 298, 780, 298, 780, 350, 810, 350); }
		metadata Route about Notation::Alignment::Phasing::'guide to collect' { points = (890, 310, 930, 310); }
		metadata Route about Notation::Alignment::Phasing::'focus to collect' { points = (890, 350, 930, 350); }
		metadata Route about Notation::Alignment::Phasing::'collect to Fine' { points = (935, 250, 935, 230, 820, 230); }
		metadata Route about Notation::Alignment::Phasing::'resume to Coarse' { points = (860, 90, 660, 90, 660, 120); }
		metadata Route about Notation::Alignment::Phasing::'recall to Fine' { points = (930, 100, 930, 140, 780, 140, 780, 200); }
		metadata Note about Notation::Alignment::Aligning { text = "guide on the current star"; x = 300; y = 250; width = 150; height = 40; }
	}
}
