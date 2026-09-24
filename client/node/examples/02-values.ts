// Every kind of value an evaluation answers with, and what the client decodes it
// to: integers exact as bigint, reals as numbers, quantities with their unit,
// enumerations as a literal, sequences of any of those.
//
//   npm run example -- 02-values

import assert from "node:assert/strict";
import { connect, formatValue, type SysMLValue } from "../src/node/index.js";
import { ROVER, section, show } from "./model.js";

/** Evaluates one expression and prints the kind it came back as. */
async function shown(
  evaluate: (expression: string) => Promise<SysMLValue>,
  expression: string,
): Promise<SysMLValue> {
  const value = await evaluate(expression);
  show(expression, `${value.kind}: ${formatValue(value)}`);
  return value;
}

async function main(): Promise<void> {
  const connection = await connect();
  try {
    const model = await connection.loads(ROVER);
    const evaluate = (expression: string): Promise<SysMLValue> => model.eval(expression);

    section("Numbers");
    // Integers decode to bigint, so a value wider than a double stays exact.
    assert.deepEqual(await shown(evaluate, "2 + 2"), { kind: "int", value: 4n });
    assert.deepEqual(await shown(evaluate, "7 / 2"), { kind: "real", value: 3.5 });
    assert.deepEqual(await shown(evaluate, "2 ** 62"), { kind: "int", value: 2n ** 62n });
    await shown(evaluate, "-17");

    section("Everything else a scalar can be");
    assert.deepEqual(await shown(evaluate, "3 > 2 and not false"), { kind: "boolean", value: true });
    assert.deepEqual(await shown(evaluate, '"curi" + "osity"'), { kind: "string", value: "curiosity" });

    section("Sequences");
    const sequence = await shown(evaluate, "(1, 2, 3)");
    assert.equal(sequence.kind, "sequence");
    assert.deepEqual(sequence.elements.map((element) => formatValue(element)), ["1", "2", "3"]);
    await shown(evaluate, '("safe", 1, 2.5, true)');

    section("The model's own values, read in the rover's scope");
    const rover = (expression: string): Promise<SysMLValue> =>
      model.eval(expression, { context: "Rover::Rover" });
    // A quantity keeps the unit it was declared with.
    const mass = await shown(rover, "mass");
    assert.equal(mass.kind, "quantity");
    assert.deepEqual(mass.magnitude, { kind: "real", value: 899 });
    show("mass unit", mass.unit);
    await shown(rover, "callsign");
    await shown(rover, "wheelCount * 2");
    const mode = await shown(rover, "mode");
    assert.equal(mode.kind, "enum");
    await shown(rover, "mode == Mode::safe");
  } finally {
    await connection.close();
  }
}

await main();
