package ModifierBeforeKind {
	part def Vehicle;

	part def Fleet {
		ref part shared : Vehicle;
		composite part owned : Vehicle;
		derived attribute count : Integer;
		ordered nonunique part queue : Vehicle[*];
		public ref part exposed : Vehicle;
		private composite part hidden : Vehicle;
	}

	action def Move {
		in part cargo : Vehicle;
		out part delivered : Vehicle;
	}
}
