import assert from 'node:assert/strict';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { chromium } from '@playwright/test';

// Run after make dev up. All requests use the real application and mock service.
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 1100 } });
  const runtimeErrors = [];
  const quoteRequests = [];
  page.on('pageerror', error => runtimeErrors.push(error.message));
  page.on('request', request => {
    if (new URL(request.url()).pathname === '/api/quotes') quoteRequests.push(request);
  });
  await page.goto('http://localhost:5173');
  assert.equal(await page.locator('h1').innerText(), 'Commission quote');
  assert.equal(await page.locator('vite-error-overlay').count(), 0);

  async function submit(amount) {
    await page.getByLabel('Loan amount (AUD)').fill(String(amount));
    await page.getByLabel('Loan term (months)').fill('36');
    await page.getByLabel('Risk band').selectOption('medium');
    const responsePromise = page.waitForResponse(response => new URL(response.url()).pathname === '/api/quotes');
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    const response = await responsePromise;
    await page.locator('form[aria-busy="false"]').waitFor();
    return { status: response.status(), body: await response.json() };
  }

  const first = await submit(10000);
  assert.equal(first.status, 200);
  assert.equal(first.body.commissionRate, 0.02);
  assert.equal(first.body.totalCommission, 200);
  assert.ok(first.body.quoteId);
  assert.deepEqual(await submit(10000), first);
  await page.reload();
  assert.deepEqual(await submit(10000), first);

  for (const amount of [100400, 100429]) {
    const failure = await submit(amount);
    assert.equal(failure.status, 500);
    assert.deepEqual(failure.body, { error: { code: 'INTERNAL_ERROR', message: 'Unable to generate a quote. Please try again later.' } });
    assert.equal(await page.getByRole('alert').innerText(), failure.body.error.message);
    assert.equal(await page.locator('.quote').count(), 0);
  }

  assert.deepEqual(await submit(10000), first);
  assert.match(await page.locator('.quote').innerText(), /AUD\s+200\.00/);
  assert.match(await page.locator('.quote').innerText(), /2%/);
  assert.equal(await page.getByRole('alert').innerText(), '');
  for (const request of quoteRequests) {
    const headers = await request.allHeaders();
    assert.equal(new URL(request.url()).origin, 'http://localhost:5173');
    assert.equal(headers['api-key'], undefined);
    assert.ok(headers['idempotency-key']);
  }
  assert.deepEqual(runtimeErrors, []);
  const screenshot = join(tmpdir(), 'commission-quote-success.png');
  await page.screenshot({ path: screenshot, fullPage: true });
  console.log('PASS: real browser → web service → quote mock; AUD 10,000 / 36 months / medium → 2%, AUD 200.00.');
  console.log('PASS: replay after reload, both simulated errors, recovery, and browser credential isolation.');
  console.log(`Screenshot: ${screenshot}`);
} finally {
  await browser.close();
}
