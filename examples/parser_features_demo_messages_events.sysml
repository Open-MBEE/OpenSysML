/*
 * Parser Features Demo: Messages & Events
 * 
 * Demonstrates SysML message and event notation:
 * - message keyword
 * - event occurrence parameters
 * - ref with body
 * 
 * Based on real stdlib patterns from Flows.sysml
 */

package MessagesAndEvents {

    // The library is not implicitly visible: a bare name needs an import.
    private import ScalarValues::*;

    
    // ===== Message Keyword =====
    
    // A message is a flow, so a flow definition types one
    abstract flow def Message {
        attribute content: String;
        attribute timestamp: Real;
    }
    
    // ===== Event Occurrences =====
    
    part def EventBasedFlow {
        // Event occurrence as parameter
        in event occurrence sourceEvent [1];
        in event occurrence targetEvent [0..1];
    }
    
    // ===== Ref with Body =====
    
    ref payload [0..*] {
        doc
        /* Reference feature with body members */
        
        attribute dataType: String;
        attribute size: Integer;
    }
    
    // ===== Combined Pattern =====
    
    part def MessageChannel {
        // Message flows
        message incoming: Message[*] nonunique;
        message outgoing: Message[*] nonunique;
        
        // Event-driven
        in event occurrence events [*];
        
        // Ref with body
        ref config [1] {
            attribute maxSize: Integer;
            attribute timeout: Real;
        }
    }
}
