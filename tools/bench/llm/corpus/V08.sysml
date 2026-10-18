package Library {
    private import StandardViewDefinitions::*;

    part def Book;
    part def Shelf {
        part books : Book[0..*];
    }
    part def ReadingRoom {
        part shelves : Shelf[1..*];
    }

    part def Library {
        doc /* A public lending library. */
        part readingRooms : ReadingRoom[1..*];
        part archive : Shelf[1..*];
    }

    requirement def CapacityRequirement {
        doc /* The library shall hold at least ten thousand books. */
    }

    view structure : BrowserView {
        expose Library::**;
    }

    view requirements : GridView {
        expose CapacityRequirement;
    }
}
