import { useState, type FormEvent } from 'react';
import Decimal from 'decimal.js';
import { generateQuote, type Quote, type QuoteRequest } from './quotes';

export default function App() {
  const [loading, setLoading] = useState(false);
  const [quote, setQuote] = useState<Quote | null>(null);
  const [error, setError] = useState('');
  const [invalidField, setInvalidField] = useState('');

  function clearFeedback() {
    setQuote(null);
    setError('');
    setInvalidField('');
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (loading) return;

    const form = event.currentTarget;
    clearFeedback();
    const invalid = form.querySelector<HTMLInputElement | HTMLSelectElement>('input:invalid, select:invalid');
    if (invalid) {
      setInvalidField(invalid.name);
      setError(`${invalid.labels?.[0]?.textContent}: ${invalid.validationMessage}`);
      invalid.focus();
      return;
    }

    // Required, min/max and step constraints reject empty/invalid fields before conversion.
    const fields = new FormData(form);
    const loanAmount = new Decimal(String(fields.get('loanAmount')));
    if (!loanAmount.isInteger() || loanAmount.lt('4000') || loanAmount.gt('10000000')) {
      setInvalidField('loanAmount');
      setError('Loan amount (AUD): Enter a whole-dollar amount between 4000 and 10000000.');
      (form.elements.namedItem('loanAmount') as HTMLInputElement).focus();
      return;
    }

    const request: QuoteRequest = {
      loanAmount,
      loanTermInMonths: Number(fields.get('loanTermInMonths')),
      riskBand: fields.get('riskBand') as QuoteRequest['riskBand'],
    };

    setLoading(true);
    const result = await generateQuote(request);
    setQuote(result.quote ?? null);
    setError(result.error ?? '');
    setLoading(false);
  }

  return (
    <>
      <header className="masthead">
        <div className="masthead-content">
          <div className="brand">
            <img src="/brand/bendigo-bank.webp" alt="Bendigo Bank" width="210" height="34" />
            <span className="workspace-name">Lending tools</span>
          </div>
          <span className="internal-label">Internal use</span>
        </div>
      </header>
      <main>
        <header className="page-heading">
          <h1>Commission quote</h1>
          <p>Enter the loan details to generate a commission quote.</p>
        </header>

        <form noValidate onSubmit={handleSubmit} onChange={clearFeedback} aria-busy={loading}>
          <fieldset disabled={loading}>
            <legend>Loan details</legend>
            <p className="form-note">All fields are required.</p>

            <div className="field">
              <label htmlFor="loanAmount">Loan amount (AUD)</label>
              <input
                id="loanAmount" name="loanAmount" type="number" inputMode="numeric"
                required min="4000" max="10000000" step="1"
                aria-invalid={invalidField === 'loanAmount'}
                aria-describedby={invalidField === 'loanAmount' ? 'amount-hint form-error' : 'amount-hint'}
              />
              <p id="amount-hint" className="hint">Whole dollars, from AUD 4,000 to AUD 10,000,000.</p>
            </div>

            <div className="field">
              <label htmlFor="loanTermInMonths">Loan term (months)</label>
              <input
                id="loanTermInMonths" name="loanTermInMonths" type="number" inputMode="numeric"
                required min="12" max="360" step="1"
                aria-invalid={invalidField === 'loanTermInMonths'}
                aria-describedby={invalidField === 'loanTermInMonths' ? 'term-hint form-error' : 'term-hint'}
              />
              <p id="term-hint" className="hint">Whole months, from 12 to 360.</p>
            </div>

            <div className="field">
              <label htmlFor="riskBand">Risk band</label>
              <select
                id="riskBand" name="riskBand" required defaultValue=""
                aria-invalid={invalidField === 'riskBand'}
                aria-describedby={invalidField === 'riskBand' ? 'form-error' : undefined}
              >
                <option value="">Select a risk band</option>
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
              </select>
            </div>

            <button type="submit">{loading ? 'Generating…' : 'Generate Quote'}</button>
          </fieldset>
        </form>

        <p id="form-error" className="error" role="alert">{error}</p>
        <p className="status" role="status" aria-atomic="true">
          {loading ? 'Generating quote…' : quote ? 'Quote generated.' : ''}
        </p>

        {quote ? (
          <section className="quote" aria-labelledby="quote-heading">
            <h2 id="quote-heading">Your commission quote</h2>
            <dl>
              <div className="total">
                <dt>Total commission</dt>
                <dd>AUD {quote.totalCommission.toFixed(2).replace(/\B(?=(\d{3})+(?!\d))/g, ',')}</dd>
              </div>
              <div>
                <dt>Commission rate</dt>
                <dd>{quote.commissionRate.times('100').toFixed(0)}%</dd>
              </div>
              <div>
                <dt>Quote ID</dt>
                <dd className="quote-id">{quote.quoteId}</dd>
              </div>
            </dl>
          </section>
        ) : null}
      </main>
      <footer className="page-footer">Bendigo Bank · Lending tools</footer>
    </>
  );
}
