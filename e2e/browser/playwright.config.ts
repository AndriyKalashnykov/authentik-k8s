import { defineConfig } from "@playwright/test";

// Headless full-login e2e for the forward-auth demo. ignoreHTTPSErrors accepts the
// self-signed dev certs (Traefik on whoami.*, Authentik on 127.0.0.1:9443).
// All timings are env-tunable with documented defaults.
export default defineConfig({
  testDir: ".",
  timeout: Number(process.env.PW_TEST_TIMEOUT_MS ?? 90000),
  expect: { timeout: Number(process.env.PW_EXPECT_TIMEOUT_MS ?? 15000) },
  fullyParallel: false,
  // 0 by default: the gate must be an honest verdict. compose-forward-auth-up now
  // waits until the login is actually reachable before the test runs, so a flaky
  // first attempt should not happen — masking it with a retry would hide a real bug.
  retries: Number(process.env.PW_RETRIES ?? 0),
  reporter: "list",
  use: {
    ignoreHTTPSErrors: true,
    headless: true,
  },
});
