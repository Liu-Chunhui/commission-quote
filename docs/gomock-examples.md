# GoMock examples

The factory `NewQuoteClient` returns `Quoter`. Handler tests can inject a generated
`MockQuoter` through `app.NewRouter` without constructing a real HTTP client.
The client tests still use `httptest` to verify HTTP behavior.

These runnable examples live in `internal/httpapi/quote_test.go`:

| Test | GoMock usage | Expected result |
| --- | --- | --- |
| `TestQuoteDecimalPrecision` | `Eq(input)`, `Return(quote, nil)`, `Times(1)` | Exact arguments, one call, HTTP 200 with unchanged decimal strings. |
| `TestQuoteDependencyError` | `Return(QuoteResponse{}, err)` | HTTP 500 with the generic error payload. |
| `TestQuoteInvalidRequest` | `Times(0)` | HTTP 400 without calling the dependency. |
| `TestQuoteRequestContext` | `DoAndReturn(...)` | Request ID, deadline, and cancellation reach the dependency. |

Each test creates `gomock.NewController(t)`, which checks expectations during test
cleanup. `gomock.Any()` accepts any argument; use an exact value or `gomock.Eq`
for arguments the test needs to verify. `DoAndReturn` callbacks must match the
mocked method's signature.

Run the examples from the repository root:

```sh
go test ./internal/httpapi -run 'TestQuote(DecimalPrecision|DependencyError|InvalidRequest|RequestContext)$' -v
```

Regenerate after changing `Quoter`:

```sh
go generate ./test/gomock
```

The directive in `test/gomock/generate.go` pins mockgen to v0.6.0 and writes
`test/gomock/quoter_mock.go`. Do not edit the generated file manually.
The first generation requires network access to download the tool.

Reference: [Uber GoMock documentation](https://github.com/uber-go/mock).
