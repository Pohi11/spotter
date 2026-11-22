// Names the requirement a test verifies, as a tag in the test's title. The JUnit reports of
// Vitest and Playwright keep titles unchanged, and tools/speccheck reads the tags from them
// (decision record 0017).

const CAPABILITY = /^[a-z][a-z0-9-]*$/;

export function spec(capability: string, requirement: string): string {
  if (!CAPABILITY.test(capability)) {
    throw new Error(`spec: capability ${JSON.stringify(capability)} is not a lowercase slug`);
  }
  if (!requirement.trim() || requirement.includes("]")) {
    throw new Error(`spec: requirement ${JSON.stringify(requirement)} is blank or contains "]"`);
  }
  return `[spec: ${capability}: ${requirement}]`;
}
