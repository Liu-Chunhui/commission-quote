import { expect, test, type Page } from '@playwright/test';
import contract from '../../api/webapi.openapi.json' with { type: 'json' };

const endpoint = '**/api/quotes';
const responses = contract.paths['/api/quotes'].post.responses;
const examples = responses['200'].content['application/json'].examples;
const standard = examples.standardQuote.value;
const failure = responses['500'].content['application/json'].example;
const validation = responses['400'].content['application/json'].example;
const expectedKey = contract['x-idempotency'].generation.example.idempotencyKey;

async function fillForm(page: Page, amount = '10000', term = '36', risk = 'medium') {
  await page.getByLabel('Loan amount (AUD)', { exact: true }).fill(amount);
  await page.getByLabel('Loan term (months)', { exact: true }).fill(term);
  await page.getByLabel('Risk band', { exact: true }).selectOption(risk);
}

test('submits typed input and the contract key, then displays the returned quote', async ({ page }) => {
  const requests: { url: string; body: unknown; headers: Record<string, string> }[] = [];
  const pageErrors: string[] = [];
  page.on('pageerror', error => pageErrors.push(error.message));
  page.on('request', request => {
    if (request.resourceType() === 'fetch') {
      requests.push({ url: request.url(), body: request.postDataJSON(), headers: request.headers() });
    }
  });
  await page.route(endpoint, route => route.fulfill({ json: standard }));
  await page.goto('/');
  await expect(page).toHaveTitle('Commission Quote');
  await expect(page.getByRole('heading', { name: 'Commission quote', exact: true })).toBeVisible();
  await expect(page.getByLabel('Loan amount (AUD)', { exact: true })).toHaveValue('');
  await expect(page.getByLabel('Loan term (months)', { exact: true })).toHaveValue('');
  await expect(page.getByLabel('Risk band', { exact: true })).toHaveValue('');
  await fillForm(page);
  await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
  await expect(page.getByText(standard.quoteId, { exact: true })).toBeVisible();
  await expect(page.getByText('2%', { exact: true })).toBeVisible();
  await expect(page.getByText('AUD 200.00', { exact: true })).toBeVisible();
  expect(requests).toHaveLength(1);
  expect(requests[0].url).toBe('http://localhost:5173/api/quotes');
  expect(requests[0].body).toEqual({ loanAmount: 10000, loanTermInMonths: 36, riskBand: 'medium' });
  expect(requests[0].headers['idempotency-key']).toBe(expectedKey);
  expect(requests[0].headers['content-type']).toBe('application/json');
  expect(requests[0].headers).not.toHaveProperty('api-key');
  expect(pageErrors).toEqual([]);
  await expect(page.locator('vite-error-overlay')).toHaveCount(0);
});

test('blocks missing, fractional and out-of-range fields without sending a request', async ({ page }) => {
  let requestCount = 0;
  await page.route(endpoint, route => {
    requestCount++;
    return route.fulfill({ json: standard });
  });
  await page.goto('/');
  for (const [amount, term, risk, field] of [
    ['', '36', 'medium', 'Loan amount'],
    ['3999', '36', 'medium', 'Loan amount'],
    ['10000001', '36', 'medium', 'Loan amount'],
    ['4000.5', '36', 'medium', 'Loan amount'],
    ['10000', '', 'medium', 'Loan term'],
    ['10000', '11', 'medium', 'Loan term'],
    ['10000', '361', 'medium', 'Loan term'],
    ['10000', '12.5', 'medium', 'Loan term'],
    ['10000', '36', '', 'Risk band'],
  ]) {
    await fillForm(page, amount, term, risk);
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByRole('alert')).toContainText(field);
    await expect(page.locator(':focus')).toHaveAttribute('aria-invalid', 'true');
  }
  await expect(page.getByLabel('Risk band', { exact: true }).locator('option')).toHaveText([
    'Select a risk band', 'Low', 'Medium', 'High',
  ]);
  expect(requestCount).toBe(0);
  await fillForm(page);
  await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
  await expect(page.getByText(standard.quoteId, { exact: true })).toBeVisible();
  await expect(page.getByRole('alert')).toBeEmpty();
  expect(requestCount).toBe(1);
});

test('accepts inclusive boundaries and every risk band', async ({ page }) => {
  const bodies: unknown[] = [];
  await page.route(endpoint, route => {
    bodies.push(route.request().postDataJSON());
    return route.fulfill({ json: standard });
  });
  await page.goto('/');
  for (const [amount, term, risk] of [
    ['4000', '12', 'low'],
    ['10000', '36', 'medium'],
    ['10000000', '360', 'high'],
  ]) {
    await fillForm(page, amount, term, risk);
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByText(standard.quoteId, { exact: true })).toBeVisible();
    expect(bodies.at(-1)).toEqual({ loanAmount: Number(amount), loanTermInMonths: Number(term), riskBand: risk });
  }
  expect(bodies).toHaveLength(3);
});

for (const [name, quote, amount, risk, rate, total] of [
  ['larger amount', examples.largerAmount.value, '750000', 'medium', '2%', 'AUD 15,000.00'],
  ['commission cents', examples.commissionCents.value, '4001', 'low', '1%', 'AUD 40.01'],
  ['server-calculated total', { quoteId: 'opaque-id', commissionRate: 0.03, totalCommission: 123.45 }, '10000', 'high', '3%', 'AUD 123.45'],
] as const) {
  test(`formats ${name} without calculating commission`, async ({ page }) => {
    await page.route(endpoint, route => route.fulfill({ json: quote }));
    await page.goto('/');
    await fillForm(page, amount, '36', risk);
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByText(quote.quoteId, { exact: true })).toBeVisible();
    await expect(page.getByText(rate, { exact: true })).toBeVisible();
    await expect(page.getByText(total, { exact: true })).toBeVisible();
  });
}

test('reuses the key after error, success, reload and a new browser session; each field changes it', async ({ page, browser }) => {
  const keys: string[] = [];
  await page.route(endpoint, route => {
    keys.push(route.request().headers()['idempotency-key']);
    return route.fulfill(keys.length === 1 ? { status: 500, json: failure } : { json: standard });
  });
  await page.goto('/');
  await fillForm(page);
  await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveText(failure.error.message);
  for (let attempt = 0; attempt < 2; attempt++) {
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByRole('status')).toContainText('Quote generated');
  }
  await page.reload();
  await fillForm(page);
  await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Quote generated');
  expect(keys).toEqual(Array(4).fill(expectedKey));
  for (const [amount, term, risk] of [
    ['10001', '36', 'medium'], ['10000', '37', 'medium'], ['10000', '36', 'high'],
  ]) {
    await fillForm(page, amount, term, risk);
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByRole('status')).toContainText('Quote generated');
  }
  expect(new Set(keys).size).toBe(4);
  for (const key of keys) expect(key).toMatch(/^[\da-f]{8}-[\da-f]{4}-5[\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/);

  const freshPage = await browser.newPage();
  try {
    let freshKey = '';
    await freshPage.route(endpoint, route => {
      freshKey = route.request().headers()['idempotency-key'];
      return route.fulfill({ json: standard });
    });
    await freshPage.goto('http://localhost:5173');
    await fillForm(freshPage);
    await freshPage.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(freshPage.getByText(standard.quoteId, { exact: true })).toBeVisible();
    expect(freshKey).toBe(expectedKey);
  } finally {
    await freshPage.close();
  }
});

test('clears stale content and prevents duplicate requests while a response is pending', async ({ page }) => {
  let requestCount = 0;
  let releaseResponse!: () => void;
  const responseReady = new Promise<void>(resolve => { releaseResponse = resolve; });
  await page.route(endpoint, async route => {
    requestCount++;
    if (requestCount === 2) await responseReady;
    await route.fulfill({ json: standard });
  });
  await page.goto('/');
  await fillForm(page);
  await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
  await expect(page.getByText(standard.quoteId, { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
  await expect(page.getByRole('status')).toHaveText('Generating quote…');
  await expect(page.getByText(standard.quoteId, { exact: true })).toHaveCount(0);
  await expect(page.getByRole('alert')).toBeEmpty();
  const button = page.getByRole('button', { name: 'Generating…', exact: true });
  await expect(button).toBeDisabled();
  await expect(page.getByLabel('Loan amount (AUD)', { exact: true })).toBeDisabled();
  await button.click({ force: true });
  await page.keyboard.press('Enter');
  await expect.poll(() => requestCount).toBe(2);
  releaseResponse();
  await expect(page.getByText(standard.quoteId, { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Generate Quote', exact: true })).toBeEnabled();
  expect(requestCount).toBe(2);
});

for (const scenario of ['validation', 'server', 'network', 'invalid JSON', 'invalid quote'] as const) {
  test(`recovers from ${scenario} without showing a stale quote`, async ({ page }) => {
    let attempt = 0;
    await page.route(endpoint, route => {
      attempt++;
      if (attempt !== 2) return route.fulfill({ json: standard });
      if (scenario === 'validation') return route.fulfill({ status: 400, json: validation });
      if (scenario === 'server') return route.fulfill({ status: 500, json: failure });
      if (scenario === 'network') return route.abort('failed');
      if (scenario === 'invalid JSON') return route.fulfill({ contentType: 'application/json', body: 'not JSON' });
      return route.fulfill({ json: { ...standard, totalCommission: '200' } });
    });
    await page.goto('/');
    await fillForm(page);
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByText(standard.quoteId, { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText(
      scenario === 'validation' ? validation.error.message : failure.error.message,
    );
    await expect(page.getByText(standard.quoteId, { exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Generate Quote', exact: true })).toBeEnabled();
    await expect(page.getByLabel('Loan amount (AUD)', { exact: true })).toHaveValue('10000');
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByText(standard.quoteId, { exact: true })).toBeVisible();
    await expect(page.getByRole('alert')).toBeEmpty();
    expect(attempt).toBe(3);
  });
}

for (const amount of ['100400', '100429']) {
  test(`submits mock trigger ${amount} normally and shows the generic API failure`, async ({ page }) => {
    let submitted: unknown;
    await page.route(endpoint, route => {
      submitted = route.request().postDataJSON();
      return route.fulfill({ status: 500, json: failure });
    });
    await page.goto('/');
    await fillForm(page, amount);
    await page.getByRole('button', { name: 'Generate Quote', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveText(failure.error.message);
    expect(submitted).toEqual({ loanAmount: Number(amount), loanTermInMonths: 36, riskBand: 'medium' });
  });
}

test('supports keyboard submission and a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 });
  await page.route(endpoint, route => route.fulfill({ json: { ...standard, quoteId: 'q'.repeat(180) } }));
  await page.goto('/');
  await page.keyboard.press('Tab');
  await expect(page.getByLabel('Loan amount (AUD)', { exact: true })).toBeFocused();
  expect(await page.locator(':focus').evaluate(element => getComputedStyle(element).outlineStyle)).toBe('solid');
  await page.keyboard.type('10000');
  await page.keyboard.press('Tab');
  await expect(page.getByLabel('Loan term (months)', { exact: true })).toBeFocused();
  await page.keyboard.type('36');
  await page.keyboard.press('Tab');
  await expect(page.getByLabel('Risk band', { exact: true })).toBeFocused();
  await page.keyboard.press('l');
  await expect(page.getByLabel('Risk band', { exact: true })).toHaveValue('low');
  await page.keyboard.press('Tab');
  await expect(page.getByRole('button', { name: 'Generate Quote', exact: true })).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status')).toContainText('Quote generated');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await expect(page.getByText('AUD 200.00', { exact: true })).toBeVisible();
});
