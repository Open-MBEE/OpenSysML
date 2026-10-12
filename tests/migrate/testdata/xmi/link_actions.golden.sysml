action def LinksModel {
    ref action clearAssociation {
        in clearInput[1];
        out clearResult[1];
    }
    ref action createLink {
        in createInput[1];
        out createResult[1];
    }
    ref action createLinkObject {
        in createObjectInput[1];
        out createObjectResult[1];
    }
    ref action destroyLink {
        in destroyInput[1];
        out destroyResult[1];
    }
    ref action readLink {
        in readInput[1];
        out readResult[1];
    }
    ref action readLinkObjectEnd {
        in readEndInput[1];
        out readEndResult[1];
    }
    ref action readLinkObjectEndQualifier {
        in readQualifierInput[1];
        out readQualifierResult[1];
    }
}
metadata MigrationMetadata::LibraryNameAvoided about LinksModel { sourceName = "Links"; }
