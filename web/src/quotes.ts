import Decimal from 'decimal.js';

const genericError = 'Unable to generate a quote. Please try again later.';
const namespace = 'eae5612f0c86500dab8a3f9eb483991f';

export type QuoteRequest = {
  loanAmount: Decimal;
  loanTermInMonths: number;
  riskBand: 'low' | 'medium' | 'high';
};

export type Quote = {
  quoteId: string;
  commissionRate: Decimal;
  totalCommission: Decimal;
};

type QuoteJSON = { quoteId: string; commissionRate: string; totalCommission: string };

type QuoteResult = { quote: Quote; error?: never } | { quote?: never; error: string };

export async function generateQuote(request: QuoteRequest): Promise<QuoteResult> {
  try {
    const key = await idempotencyKey(request);
    const response = await fetch('/api/quotes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'idempotency-key': key },
      body: JSON.stringify(request),
    });
    const body = await response.json();

    if (response.status === 400 && body?.error?.code === 'INVALID_REQUEST'
      && typeof body.error.message === 'string' && body.error.message.trim()) {
      return { error: body.error.message };
    }

    if (response.status !== 200 || !isQuote(body)) {
      return { error: genericError };
    }

    return { quote: {
      quoteId: body.quoteId,
      commissionRate: new Decimal(body.commissionRate),
      totalCommission: new Decimal(body.totalCommission),
    } };
  } catch {
    return { error: genericError };
  }
}

async function idempotencyKey(request: QuoteRequest): Promise<string> {
  const name = `${request.loanAmount.toFixed(0)}|${request.loanTermInMonths}|${request.riskBand}`;
  const namespaceBytes = Uint8Array.from(namespace.match(/../g)!, hex => parseInt(hex, 16));
  const input = new Uint8Array([...namespaceBytes, ...new TextEncoder().encode(name)]);
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-1', input)).slice(0, 16);

  // UUIDv5 uses the namespace/name SHA-1 digest with RFC version and variant bits.
  digest[6] = (digest[6] & 0x0f) | 0x50;
  digest[8] = (digest[8] & 0x3f) | 0x80;
  const hex = Array.from(digest, byte => byte.toString(16).padStart(2, '0')).join('');
  return [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join('-');
}

function isQuote(value: unknown): value is QuoteJSON {
  return typeof value === 'object' && value !== null
    && 'quoteId' in value && typeof value.quoteId === 'string' && value.quoteId.length > 0
    && 'commissionRate' in value && typeof value.commissionRate === 'string'
    && ['0.01', '0.02', '0.03'].includes(value.commissionRate)
    && 'totalCommission' in value && typeof value.totalCommission === 'string'
    && /^-?(0|[1-9][0-9]*)(\.[0-9]{1,2})?$/.test(value.totalCommission);
}
