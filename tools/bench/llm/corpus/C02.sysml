package Rectangle {
    private import ISQ::*;
    private import SI::*;

    calc def Area {
        in width : LengthValue;
        in height : LengthValue;
        return area : AreaValue = width * height;
    }

    part def Table {
        attribute width : LengthValue = 1.2 [m];
        attribute depth : LengthValue = 0.8 [m];
        attribute surface : AreaValue = Area(width, depth);
    }
}
