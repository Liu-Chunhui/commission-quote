import assert from 'node:assert/strict';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { chromium } from '@playwright/test';

// Use loanAmount failure mode. APP_URL also allows checking the container stack.
const appURL = process.env.APP_URL ?? 'http://localhost:5173';
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 1100 } });
  const runtimeErrors = [];
  const quoteRequests = [];
  page.on('pageerror', error => runtimeErrors.push(error.message));
  page.on('request', request => {
    if (new URL(request.url()).pathname === '/api/quotes') quoteRequests.push(request);
  });
  await page.goto(appURL);
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

  const first = await submit('10000');
  assert.equal(first.status, 200);
  assert.equal(first.body.commissionRate, '0.02');
  assert.equal(first.body.totalCommission, '200');
  assert.ok(first.body.quoteId);
  assert.deepEqual(await submit('10000'), first);
  await page.reload();
  assert.deepEqual(await submit('10000'), first);

  const cents = await submit('10003');
  assert.equal(cents.status, 200);
  assert.equal(cents.body.totalCommission, '200.06');

  for (const amount of ['100400', '100401', '100409', '100429', '100500', '100503']) {
    const failure = await submit(amount);
    assert.equal(failure.status, 500);
    assert.deepEqual(failure.body, { error: { code: 'INTERNAL_ERROR', message: 'Unable to generate a quote. Please try again later.' } });
    assert.equal(await page.getByRole('alert').innerText(), failure.body.error.message);
    assert.equal(await page.locator('.quote').count(), 0);
  }

  assert.deepEqual(await submit('10000'), first);
  assert.match(await page.locator('.quote').innerText(), /AUD\s+200\.00/);
  assert.match(await page.locator('.quote').innerText(), /2%/);
  assert.equal(await page.getByRole('alert').innerText(), '');
  for (const request of quoteRequests) {
    const headers = await request.allHeaders();
    assert.equal(new URL(request.url()).origin, new URL(appURL).origin);
    assert.equal(typeof request.postDataJSON().loanAmount, 'string');
    assert.equal(headers['api-key'], undefined);
    assert.ok(headers['idempotency-key']);
  }
  assert.deepEqual(runtimeErrors, []);
  const screenshot = join(tmpdir(), 'commission-quote-success.png');
  await page.screenshot({ path: screenshot, fullPage: true });
  console.log('PASS: real browser → web service → quote mock; AUD 10,000 / 36 months / medium → 2%, AUD 200.00.');
  console.log('PASS: exact cents, replay after reload, all six simulated errors, recovery, and browser credential isolation.');
  console.log(`Screenshot: ${screenshot}`);
} finally {
  await browser.close();
}
