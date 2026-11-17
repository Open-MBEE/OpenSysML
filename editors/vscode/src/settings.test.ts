import assert from "node:assert/strict";
import { test } from "node:test";

import * as manifest from "../package.json";
import { SERVER_SETTING_SECTIONS, serverSettings } from "./settings";

function configuration(values: Record<string, unknown>) {
  return {
    get<T>(key: string, defaultValue: T): T {
      return key in values ? (values[key] as T) : defaultValue;
    },
  };
}

test("serverSettings defaults to the statement layout and ten owned members", () => {
  assert.deepEqual(serverSettings(configuration({})), {
    sysml: { keepLineBreaks: false, hoverMemberLimit: 10, hoverInheritedMembers: false },
  });
});

test("serverSettings forwards opensysml.format.keepLineBreaks", () => {
  assert.deepEqual(serverSettings(configuration({ "format.keepLineBreaks": true })).sysml.keepLineBreaks, true);
});

test("serverSettings forwards the hover member settings", () => {
  const settings = serverSettings(configuration({ "hover.memberLimit": 0, "hover.inheritedMembers": true }));
  assert.deepEqual(settings.sysml, { keepLineBreaks: false, hoverMemberLimit: 0, hoverInheritedMembers: true });
});

test("the hover settings are declared with the defaults serverSettings falls back to", () => {
  const properties = manifest.contributes.configuration.properties;
  assert.equal(properties["opensysml.hover.memberLimit"].default, serverSettings(configuration({})).sysml.hoverMemberLimit);
  assert.equal(properties["opensysml.hover.memberLimit"].minimum, 0);
  assert.equal(properties["opensysml.hover.inheritedMembers"].default, false);
  for (const key of Object.keys(properties).filter((name) => /^opensysml\.(format|hover)\./.test(name))) {
    assert.ok(SERVER_SETTING_SECTIONS.some((section) => key.startsWith(section + ".")), key);
  }
});
