package Ordered {
    part def Shelf { ref part content[*]; }
    part def OrderedShelf :> Shelf {
        orderedContent ordered :>> content;
        bagContent nonunique :>> content;
        listContent ordered nonunique :>> content;
        items ordered;
    }
}
