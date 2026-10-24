// Prefix metadata between `individual` and `def` or the usage keyword
// (SysML.xtext IndividualDefinition, IndividualUsage); the trailing two
// members put it before `individual`, which no production spells.
package IndividualPrefixMetadata {
    metadata def Tag;
    occurrence def Campaign;
    part def Body;
    individual #Tag def Flight :> Campaign;
    abstract individual #Tag def Sortie :> Campaign;
    individual #Tag part def Article :> Body;
    individual #Tag flight : Flight;
    individual #Tag part article : Article;
    individual #Tag occurrence sortie : Sortie;
    #Tag individual part def Spare :> Body;
    #Tag individual part spare : Spare;
}
