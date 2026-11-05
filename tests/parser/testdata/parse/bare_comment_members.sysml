/* A comment before the first member annotates the root namespace. */
package BareComments {
	/* A comment at a member position is an anonymous Comment member
	 * of the package, the `comment` keyword omitted. */
	part def Vehicle {
		doc /* The body of a documentation. */
		/* A comment after a documentation is a member of its own. */
		// A note stays trivia of the comment it precedes.
		part eng;
		part /* a comment inside a declaration head stays trivia */ wheel;
		/* A comment before the closing brace is a member too. */
	}
	action def Drive {
		action start;
		/* A comment before a member of an action body. */
		then done;
	}
	calc def Mass { attribute m : Real; /* before the result */ m }
	/* A comment at the end of the package body. */
}
/* A comment at the end of the file annotates the root namespace. */
