// Executing actions and state machines under scheduling policies, exploring
// their choice points, and performing on a nested subject.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import { InvalidRequestError, connect, formatValue } from "../src/node/index.js";
import type { SysMLValue } from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const SCHEDULE_MODEL = `package Sched {
    private import ScalarValues::*;

    action race {
        attribute winner : Integer = 0;
        first start;
        fork split;
        action left { assign winner := 1; }
        action right { assign winner := 2; }
        join sync;
        done;
        succession first start then split;
        succession first split then left;
        succession first split then right;
        succession first left then sync;
        succession first right then sync;
        succession first sync then done;
    }

    state Dispatcher {
        attribute level : Integer = 8;
        entry; then idle;
        state idle;
        state low;
        state high;
        transition first idle accept Go if level > 5 then low;
        transition first idle accept Go if level > 7 then high;
    }

    action def Race {
        out winner : Integer = 0;
        first start;
        fork split;
        action left { assign winner := 1; }
        action right { assign winner := 2; }
        join sync;
        done;
        succession first start then split;
        succession first split then left;
        succession first split then right;
        succession first left then sync;
        succession first right then sync;
        succession first sync then done;
    }

    analysis raced {
        out winner : Integer;
        perform action race : Race;
        return : Integer = winner;
    }
}
`;

const PERFORMER_MODEL = `package Wire {
    private import ScalarValues::*;
    item def Ping;
    port def Link { in item ping : Ping; }
    part def Ground {
        port p : ~Link;
        exhibit state hail { entry; then go; state go { entry send new Ping() via p; } }
    }
    part def Craft {
        port p : Link;
        attribute pinged : Boolean = false;
        exhibit state modes {
            entry; then waiting;
            state waiting;
            transition first waiting accept Ping via p then active;
            state active { entry assign pinged := true; }
        }
        action look { out seen : Boolean; first start; then action read assign seen := pinged; then done; }
    }
    part def Pair {
        part ground : Ground;
        part craft : Craft;
        connect craft.p to ground.p;
    }
    part pair : Pair;
}
`;

test("the service advertises schedule, explore and performer", async () => {
  await using connection = await connect();
  assert.ok(connection.info.has("schedule"));
  assert.ok(connection.info.has("schedule_explore"));
  assert.ok(connection.info.has("performer"));
});

test("the policy selects which write stands", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  const plain = await model.executeAction("Sched::race");
  assert.deepEqual(plain.outputs.get("winner"), { kind: "int", value: 1n });
  const reverse = await model.executeAction("Sched::race", { schedule: "reverse" });
  assert.deepEqual(reverse.outputs.get("winner"), { kind: "int", value: 1n });
  const declared = await model.executeAction("Sched::race", { schedule: "declared" });
  assert.deepEqual(declared.outputs.get("winner"), { kind: "int", value: 2n });
});

test("the policy selects the transition taken", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  const declared = await model.executeState("Sched::Dispatcher", {
    events: ["Go"],
    schedule: "declared",
  });
  assert.deepEqual(declared.statesVisited, ["idle", "low"]);
  const seeded = await model.executeState("Sched::Dispatcher", {
    events: ["Go"],
    schedule: "seed:3",
  });
  assert.deepEqual(seeded.statesVisited, ["idle", "high"]);
});

test("an analysis performs its actions under the policy", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  const result = await model.runAnalysis("Sched::raced", { schedule: "declared" });
  assert.deepEqual(result.outputs.get("winner"), { kind: "int", value: 2n });
});

test("the same seed answers the same way", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  const answers = new Set<unknown>();
  for (let i = 0; i < 3; i += 1) {
    const run = await model.executeAction("Sched::race", { schedule: "seed:11" });
    answers.add(formatValue(run.outputs.get("winner") as SysMLValue));
  }
  assert.equal(answers.size, 1);
});

test("a spelling naming no policy is an invalid request", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  for (const spelling of ["random", "seed", "seed:", "seed:-1", "seed:abc"]) {
    await assert.rejects(
      () => model.executeAction("Sched::race", { schedule: spelling }),
      /scheduling policy|InvalidRequestError|invalid/i,
    );
  }
});

test("both writers are reached and the exploration is complete", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  const exploration = await model.exploreAction("Sched::race");
  assert.deepEqual(
    exploration.outcomes
      .map((o) => (o.outputs.get("winner") as { value: bigint }).value)
      .sort(),
    [1n, 2n],
  );
  assert.ok(exploration.outcomes.every((o) => o.linearizations === 1));
  assert.equal(exploration.status, "complete (2 runs)");
  assert.equal(exploration.complete, true);
});

test("both transitions are reached", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  const exploration = await model.exploreState("Sched::Dispatcher", { events: ["Go"] });
  assert.deepEqual(
    exploration.outcomes.map((o) => o.finalState).sort(),
    ["high", "low"],
  );
  assert.equal(exploration.complete, true);
});

test("an analysis explores the actions it performs", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  const exploration = await model.exploreAnalysis("Sched::raced");
  assert.deepEqual(
    exploration.outcomes
      .map((o) => (o.outputs.get("winner") as { value: bigint }).value)
      .sort(),
    [1n, 2n],
  );
  assert.equal(exploration.complete, true);
});

test("a runs budget of one is incomplete", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  const exploration = await model.exploreAction("Sched::race", { schedule: "explore:runs=1" });
  assert.equal(exploration.length, 1);
  assert.equal(
    exploration.status,
    "incomplete: runs budget 1 hit after 1 runs; probabilities are lower bounds",
  );
});

test("the single-run methods refuse an exploring schedule", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  await assert.rejects(
    () => model.executeAction("Sched::race", { schedule: "explore" }),
    /exploreAction/,
  );
  await assert.rejects(
    () => model.runAnalysis("Sched::raced", { schedule: "explore" }),
    /exploreAnalysis/,
  );
});

test("the exploring methods refuse a non-exploring schedule", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  await assert.rejects(
    () => model.exploreAction("Sched::race", { schedule: "declared" }),
    /explore/,
  );
  await assert.rejects(
    () => model.exploreAnalysis("Sched::raced", { schedule: "seed:1" }),
    /explore/,
  );
});

test("malformed exploring spellings are an invalid request", async () => {
  await using connection = await connect();
  const model = await connection.loads(SCHEDULE_MODEL);
  for (const spelling of ["explore:", "explore:runs=0", "explore:depth=-1", "explore:runs=x"]) {
    await assert.rejects(
      () => model.exploreAction("Sched::race", { schedule: spelling }),
      InvalidRequestError,
    );
  }
});

test("the machine of a nested part hears its sibling", async () => {
  await using connection = await connect();
  const model = await connection.loads(PERFORMER_MODEL);
  const run = await model.executeState("Wire::Craft::modes", { performer: "Wire::pair.craft" });
  assert.equal(run.statesVisited[run.statesVisited.length - 1], "active");
  const alone = await model.executeState("Wire::Craft::modes", { performer: "Wire::Craft" });
  assert.equal(alone.statesVisited[alone.statesVisited.length - 1], "waiting");
});

test("an explored run makes the assembly anew", async () => {
  await using connection = await connect();
  const model = await connection.loads(PERFORMER_MODEL);
  const exploration = await model.exploreState("Wire::Craft::modes", {
    performer: "Wire::pair.craft",
  });
  assert.deepEqual(
    exploration.outcomes.map((o) => o.finalState),
    ["active"],
  );
  assert.equal(exploration.status, "complete (1 runs)");
});
