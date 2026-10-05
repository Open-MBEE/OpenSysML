action def LinksModel {
    action clearAssociation {
        in clearInput[1];
        out clearResult[1];
    }
    action createLink {
        in createInput[1];
        out createResult[1];
    }
    action createLinkObject {
        in createObjectInput[1];
        out createObjectResult[1];
    }
    action destroyLink {
        in destroyInput[1];
        out destroyResult[1];
    }
    action readLink {
        in readInput[1];
        out readResult[1];
    }
    action readLinkObjectEnd {
        in readEndInput[1];
        out readEndResult[1];
    }
    action readLinkObjectEndQualifier {
        in readQualifierInput[1];
        out readQualifierResult[1];
    }
}
metadata MigrationMetadata::LibraryNameAvoided about LinksModel { sourceName = "Links"; }
