package EndPrefixMetadata {
    metadata def original;
    metadata def derive;
    requirement def Req1;
    requirement def Req1_1;
    #derive connection def Derivation {
        end #original r1 : Req1;
        end #derive r1_1 : Req1_1;
        #derive end r1_2 : Req1_1;
        end [*] ref cause : Req1;
        end ref attribute effect : Req1_1[*];
    }
    part context {
        requirement req1 : Req1;
        requirement req1_1 : Req1_1;
        #derive connection : Derivation {
            end r1 ::> req1;
            end r1_1 references req1_1;
        }
    }
}
