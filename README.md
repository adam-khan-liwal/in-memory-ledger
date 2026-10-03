# In-Memory Account Ledger Core

This suite processes a sequential event stream of financial transactions across multiple days, managing available vs. ledger balances, delayed processing (late-arriving events), end-of-day overdraft fees, and daily interest accruals.

## How to Run
Ensure you have Go installed (1.18+ recommended).
Run the suite directly using:
```bash
go run main.go