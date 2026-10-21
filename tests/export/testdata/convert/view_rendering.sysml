package Views {
    rendering def Tree;
    rendering tree : Tree;
    view def ByReference {
        render tree;
    }
    view def ByDeclaration {
        render rendering t2 : Tree;
    }
    view def WithBody {
        render tree {
            doc /* The tree rendering. */
        }
    }
}
