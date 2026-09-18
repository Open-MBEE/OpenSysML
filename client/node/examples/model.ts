// The model every example works on, and the printing each of them shares. It is
// one model rather than a snippet per example so that what one example shows can
// be looked for in the next: a rover, its wheels, its battery and its cameras.

/** The rover model, written in SysML v2 notation the service parses clean. */
export const ROVER = `package Rover {
  enum def Mode {
    safe;
    science;
    driving;
  }

  part def Wheel {
    attribute radius : ISQ::LengthValue = 0.25 [SI::m];
    attribute treads : ScalarValues::Integer = 24;
  }

  part def Battery {
    attribute capacity : ScalarValues::Real = 42.0;
    attribute chemistry : ScalarValues::String = "Li-ion";
    attribute charged : ScalarValues::Boolean = true;
  }

  part def Camera {
    attribute megapixels : ScalarValues::Integer = 12;
    // Declared and never given a value: an instance reports it unset.
    attribute calibrated : ScalarValues::Boolean;
  }

  part def Rover {
    part wheels : Wheel[6];
    part battery : Battery;
    part cameras : Camera[0..4];
    attribute mass : ISQ::MassValue = 899.0 [SI::kg];
    attribute mode : Mode = Mode::safe;
    attribute callsign : ScalarValues::String = "curiosity";
    attribute wheelCount : ScalarValues::Integer = 6;
  }

  part def HeavyRover :> Rover {
    attribute ballast : ScalarValues::Real = 120.0;
  }

  part fleet : Rover[2];
}
`;

/** Prints one labelled line, so an example's output reads as a transcript. */
export function show(label: string, value: unknown): void {
  console.log(`  ${label.padEnd(34)} ${String(value)}`);
}

/** Prints a heading for the section of the example that follows. */
export function section(title: string): void {
  console.log(`\n${title}`);
}
