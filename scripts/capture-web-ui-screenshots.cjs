#!/usr/bin/env node
/*
 * Capture the Authentik Web UI screenshots referenced by docs/web-ui.md against
 * a LIVE Authentik instance (Docker Compose or KinD), with the org-01/org-02
 * demo data the provisioner creates.
 *
 * Prerequisites (one-time):
 *   npm i playwright && npx playwright install chromium
 *   # or reuse an existing Playwright install:
 *   #   node scripts/capture-web-ui-screenshots.cjs   (requires 'playwright' resolvable)
 *
 * Stand up + provision the stack first (from provisioner/):
 *   make compose-up && make run     # Compose on https://localhost:9443
 *
 * Then capture (all values are env-overridable, defaults mirror the demo stack):
 *   AK_BASE=https://localhost:9443 node scripts/capture-web-ui-screenshots.cjs
 *
 * Output: JPEGs written straight to docs/img/ (deviceScaleFactor=2, type:jpeg).
 *
 * Why Playwright (not plain `chrome --headless --screenshot`): the admin UI is a
 * shadow-DOM web-component SPA behind a multi-step login flow. Playwright pierces
 * open shadow roots with normal CSS selectors, interacts (type username -> submit
 * -> type password), waits for SPA route data to settle, and skips the self-signed
 * cert via ignoreHTTPSErrors. A single `--screenshot` shot cannot do any of that.
 */
const path = require('path');
const fs = require('fs');
const { chromium } = require('playwright');

const BASE = process.env.AK_BASE || 'https://localhost:9443';
const OUT = process.env.AK_OUT || path.join(__dirname, '..', 'docs', 'img');
const ADMIN_USER = process.env.AK_ADMIN_USER || 'akadmin';
const ADMIN_PASS = process.env.AK_ADMIN_PASS || 'Authentik01234567890!';
const DEMO_USER = process.env.AK_DEMO_USER || 'org-01-admin';   // a provisioned user
const DEMO_GROUP = process.env.AK_DEMO_GROUP || 'org-01-admins'; // a provisioned group
const SETTLE_MS = Number(process.env.AK_SETTLE_MS || '2800');    // SPA render settle
fs.mkdirSync(OUT, { recursive: true });

const log = (...a) => console.log('[shots]', ...a);
const shot = async (page, name, settle = SETTLE_MS) => {
  await page.waitForTimeout(settle);
  await page.screenshot({ path: path.join(OUT, `${name}.jpg`), type: 'jpeg', quality: 88 });
  log('captured', name);
};

// Click a tab by visible label; tries several strategies (pierces shadow DOM).
async function clickTab(page, label) {
  for (const loc of [
    page.getByRole('tab', { name: label, exact: true }),
    page.locator(`button:has-text("${label}")`),
    page.locator(`a:has-text("${label}")`),
    page.getByText(label, { exact: true }),
  ]) {
    try { const el = loc.first(); if (await el.count()) { await el.click({ timeout: 3000 }); return true; } }
    catch (_) { /* next strategy */ }
  }
  return false;
}

// Open a directory row whose name cell is either `<a><div>name</div></a>`
// (users list) or `<a>name</a>` (groups list).
async function openRow(page, name) {
  await page.locator(`a:text-is("${name}"), a:has(div:text-is("${name}"))`).first().click({ timeout: 5000 });
  await page.waitForTimeout(2500);
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    ignoreHTTPSErrors: true,           // self-signed dev cert
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,              // retina-crisp output
  });
  const page = await ctx.newPage();

  // 1. login — identification step
  await page.goto(`${BASE}/if/flow/default-authentication-flow/`, { waitUntil: 'networkidle' });
  await shot(page, 'login', 2000);

  // 2. password — type username, submit, capture the password step
  await page.fill('input[name="uidField"]', ADMIN_USER);
  await page.click('button[type="submit"]');
  await shot(page, 'password', 2000);

  // complete login + enter the admin interface
  await page.fill('input[name="password"]', ADMIN_PASS);
  await page.click('button[type="submit"]');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(2500);
  await page.goto(`${BASE}/if/admin/`, { waitUntil: 'networkidle' });
  await page.waitForTimeout(3000);

  // 3. users list
  await page.goto(`${BASE}/if/admin/#/identity/users`, { waitUntil: 'networkidle' });
  await shot(page, 'users', 3000);

  // 4. users-groups — open a user, click its Groups tab
  await openRow(page, DEMO_USER);
  log('users-groups tab:', await clickTab(page, 'Groups'));
  await shot(page, 'users-groups');

  // 5. groups list
  await page.goto(`${BASE}/if/admin/#/identity/groups`, { waitUntil: 'networkidle' });
  await shot(page, 'groups', 3000);

  // 6. groups-users — open a group, click its Users tab
  await openRow(page, DEMO_GROUP);
  log('groups-users tab:', await clickTab(page, 'Users'));
  await shot(page, 'groups-users');

  // 7. tokens list
  await page.goto(`${BASE}/if/admin/#/core/tokens`, { waitUntil: 'networkidle' });
  await shot(page, 'tokens', 3000);

  await browser.close();
  log('DONE -> ' + OUT);
})().catch((e) => { console.error('FATAL', e); process.exit(1); });
