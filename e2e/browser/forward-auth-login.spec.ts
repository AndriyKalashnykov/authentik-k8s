import { test, expect } from "@playwright/test";

// End-to-end proof that the forward-auth demo works for a real browser user:
//   browse whoami -> Traefik forwardAuth bounces to the Authentik login ->
//   sign in -> land back on whoami, which echoes the X-authentik-* identity headers.
//
// Env-driven (defaults mirror compose/.env.example). AK_ADMIN_PASS is required and
// supplied via the environment (never argv) by `make e2e-forward-auth-browser`.
const APP_URL = process.env.APP_URL ?? "https://whoami.127-0-0-1.sslip.io/";
const ADMIN_USER = process.env.AK_ADMIN_USER ?? "akadmin";
const ADMIN_PASS = process.env.AK_ADMIN_PASS ?? "";
const LAND_TIMEOUT_MS = Number(process.env.PW_LAND_TIMEOUT_MS ?? 45000);

test("forward-auth: a browser user logs in via Authentik and whoami serves the identity headers", async ({ page }) => {
  expect(ADMIN_PASS, "AK_ADMIN_PASS must be set (the AUTHENTIK_BOOTSTRAP_PASSWORD)").not.toBe("");

  // 1. Browse the protected app — unauthenticated, so forwardAuth redirects to Authentik.
  await page.goto(APP_URL, { waitUntil: "domcontentloaded" });

  // 1b. DENY invariant (the security property, not just the happy path): before
  //     authenticating, whoami MUST NOT be served — forwardAuth intercepts the
  //     unauth request and we land on the Authentik login. Assert the login field
  //     is present AND that whoami's body/identity content is absent, so a broken
  //     or open middleware (serving whoami without auth) fails loudly here.
  await page.waitForSelector('input[name="uidField"]', { state: "visible" });
  const preLoginBody = await page.evaluate(() => document.body?.innerText ?? "");
  expect(preLoginBody, "whoami identity header must NOT be served before login").not.toMatch(
    /X-Authentik-Username:/i,
  );
  expect(preLoginBody, "whoami page body (Hostname:) must NOT be served before login").not.toMatch(
    /Hostname:/,
  );

  // 2. Identification stage. The flow UI is a shadow-DOM SPA; Playwright pierces open shadow roots.
  await page.fill('input[name="uidField"]', ADMIN_USER);
  await page.click('button[type="submit"]');

  // 3. Password stage. Wait for it to render, then verify the value actually took —
  //    the SPA can swap the input element mid-fill, leaving it empty.
  await page.waitForTimeout(2500);
  await page.waitForSelector('input[name="password"]', { state: "visible" });
  let filled = "";
  for (let i = 0; i < 5 && filled !== ADMIN_PASS; i++) {
    await page.fill('input[name="password"]', ADMIN_PASS);
    await page.waitForTimeout(400);
    filled = await page.inputValue('input[name="password"]');
  }
  expect(filled, "password field populated").toHaveLength(ADMIN_PASS.length);
  await page.click('button[type="submit"]');

  // 4. The OAuth round-trip lands back on the whoami host, whose plain-text page prints "Hostname:".
  //    Let the redirect chain settle, then poll — and tolerate mid-navigation: a page.evaluate that
  //    races a redirect throws "Execution context was destroyed", which expect.poll treats as a hard
  //    failure, so catch it and keep polling until the page settles on whoami.
  await page.waitForLoadState("networkidle").catch(() => {});
  await expect
    .poll(
      async () => {
        try {
          const text = await page.evaluate(() => document.body?.innerText ?? "");
          return /Hostname:/.test(text) && /whoami\.127-0-0-1\.sslip\.io/.test(page.url());
        } catch {
          return false; // mid-navigation (context destroyed) — retry
        }
      },
      { timeout: LAND_TIMEOUT_MS, intervals: [1000] },
    )
    .toBe(true);

  // 5. The end result: whoami is served (not the login) AND carries the identity
  //    headers Authentik injected through the forwardAuth middleware.
  const body = await page.evaluate(() => document.body.innerText);
  expect(page.url()).toContain("whoami.127-0-0-1.sslip.io");
  expect(body, "whoami should echo the authenticated identity header").toMatch(/X-Authentik-Username:\s*\S/i);
});
