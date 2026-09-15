const assert = require("node:assert/strict");
const { removeUnsafeExactAliases } = require("./field_alias_safety.js");

const aliases = {
  full_name: ["full name", "name"],
  first_name: ["first name", "given name"],
  location: ["current location", "location", "city"],
};

removeUnsafeExactAliases(aliases);

assert.deepEqual(aliases.full_name, ["full name"]);
assert.deepEqual(aliases.first_name, ["first name", "given name"]);
assert.deepEqual(aliases.location, ["current location", "location", "city"]);

// Idempotent so extension reload/order changes cannot corrupt the alias table.
removeUnsafeExactAliases(aliases);
assert.deepEqual(aliases.full_name, ["full name"]);

console.log("browser companion field alias safety tests passed");
