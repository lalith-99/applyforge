const assert = require("node:assert/strict");
const { removeUnsafeExactAliases } = require("./field_alias_safety.js");

const aliases = {
  full_name: ["full name", "name"],
  first_name: ["first name", "given name"],
  location: ["current location", "location", "city"],
  desired_location: ["desired location", "preferred location"],
};

removeUnsafeExactAliases(aliases);

assert.deepEqual(aliases.full_name, ["full name"]);
assert.deepEqual(aliases.first_name, ["first name", "given name"]);
assert.deepEqual(aliases.location, ["current location"]);
assert.deepEqual(aliases.desired_location, ["desired location", "preferred location"]);

// Idempotent so extension reload/order changes cannot corrupt the alias table.
removeUnsafeExactAliases(aliases);
assert.deepEqual(aliases.full_name, ["full name"]);
assert.deepEqual(aliases.location, ["current location"]);

console.log("browser companion field alias safety tests passed");
