import { expect, test } from "vitest";

import { spec } from "./spec";

test("builds the tag speccheck reads", () => {
  expect(spec("dashboard", "Results come from dated reports")).toBe("[spec: dashboard: Results come from dated reports]");
});

test("rejects a malformed capability or requirement", () => {
  expect(() => spec("Dash Board", "x")).toThrow();
  expect(() => spec("dashboard", " ")).toThrow();
  expect(() => spec("dashboard", "a]b")).toThrow();
});
