package 'esc\'aped' {
	part def Pt;
	part def Holder {
		part 'it\'s' : Pt;
		part 'a\\b' : Pt;
		part 'q\"d' : Pt;
		part 'tab\there' : Pt;
		part 'line\nfeed' : Pt;
		part 'bell\bform\fret\rurn' : Pt;
		part 'x::y' : Pt;
		part <'sh\'ort'> 'two words' : Pt;
	}
	part h : Holder {
		part redefines 'it\'s';
		part :>> Holder::'a\\b';
	}
	alias Al for Holder::'it\'s';
	attribute n = h.'it\'s';
}
