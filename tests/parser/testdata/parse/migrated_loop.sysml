package P {
	action def Looping {
		in feed : ScalarValues::Integer;
		out 'out' : ScalarValues::Integer;
		first start then 'loop';
		action 'loop' {
			in iIn : ScalarValues::Integer;
			in sIn : ScalarValues::Integer;
			out iOut : ScalarValues::Integer;
			out sOut : ScalarValues::Integer;
			private attribute i : ScalarValues::Integer := iIn;
			private attribute s : ScalarValues::Integer := setup.zero.r;
			private attribute ended : ScalarValues::Boolean := false;
			first start then setup;
			action setup {
				first start then zero;
				action zero {
					out r : ScalarValues::Integer;
					assign r := 0;
				}
			}
			then action iterate {
				loop {
					action test {
						first start then le;
						action le {
							in x : ScalarValues::Integer = i;
							in y : ScalarValues::Integer = s;
							out r : ScalarValues::Boolean;
							out k : ScalarValues::Integer;
							assign r := x <= y;
							assign k := 10 * x;
						}
					}
					then if not test.le.r {
						assign ended := true;
					} else {
						action body {
							first start then plus;
							action plus {
								in x : ScalarValues::Integer = s;
								in y : ScalarValues::Integer = test.le.k;
								out r : ScalarValues::Integer;
								assign r := x + y;
							}
							first plus then inc;
							action inc {
								in x : ScalarValues::Integer = i;
								out r : ScalarValues::Integer;
								assign r := x + 1;
							}
						}
						then action next {
							assign i := body.inc.r;
							then assign s := body.plus.r;
						}
					}
				} until ended;
			}
			then action results {
				assign iOut := i;
				then assign sOut := s;
			}
		}
		first 'loop' then final;
		action final terminate;
		bind 'loop'.iIn = feed;
		bind 'loop'.sIn = feed;
		bind 'out' = 'loop'.sOut;
	}
	action def PostLoop {
		first start then post;
		action post {
			in xIn[0..*] nonunique;
			out xOut[0..*] nonunique;
			private ref x[0..*] nonunique := xIn;
			private attribute ended : ScalarValues::Boolean := false;
			first start then iterate;
			then action iterate {
				loop {
					action body {
						first start then grow;
						action grow {
							in v[0..*] nonunique = x;
							out r[0..*] nonunique;
						}
					}
					then action next {
						assign x := body.grow.r;
					}
					then action test {
						first start then lt;
						action lt {
							in v[0..*] nonunique = x;
							out r : ScalarValues::Boolean;
						}
					}
					then if not test.lt.r {
						assign ended := true;
					}
				} until ended;
			}
			then action results {
				assign xOut := x;
			}
		}
		first post then final;
		action final terminate;
	}
}
