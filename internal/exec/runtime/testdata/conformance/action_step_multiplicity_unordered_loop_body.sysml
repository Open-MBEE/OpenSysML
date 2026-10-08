package test {
	action def U {
		first start then worker;
		action worker {
			while true {
				action anchor;
				first start then anchor;
				action tick[3] { }
			}
		}
		then done;
	}
}
