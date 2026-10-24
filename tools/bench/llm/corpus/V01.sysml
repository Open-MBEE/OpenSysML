package Garden {
    part def Shed {
        doc /* A wooden shed that stores the garden tools. */
    }
    part def Greenhouse {
        doc /* A glass greenhouse for seedlings. */
    }
    part def Garden {
        doc /* The whole back garden with its two buildings. */
        part shed : Shed;
        part greenhouse : Greenhouse;
    }
}
