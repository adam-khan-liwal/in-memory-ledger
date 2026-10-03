# Ledger Constants

* **`25` (Overdraft Fee in AED):** Explicitly dictated by requirements. Halving this to 12.50 would violate the business rule. In the code, it is multiplied by the currency multiplier (`10^2`) to store as 2500 minor units.
* **`0.0004` (Daily Interest Rate):** Represents the 0.04% daily interest specified in the prompt. Halving this to 0.0002 (0.02%) would calculate incorrect accruals.
* **`2` and `3` (Currency Precisions):** AED operates in 2 decimal places (fils); BHD operates in 3 decimal places (fils). Halving these is mathematically impossible for base-10 exponent multipliers and would destroy currency formatting.
* **`1` (Day Increment):** The ledger advances exactly 1 day at a time (`AddDate(0, 0, 1)`) when looking for missing EOD gaps. Halving this (12 hours) breaks the End-Of-Day batch processing standard.
* **`10.000` -> `[3.334, 3.333, 3.333]` (BHD Installments):** The requirement to split BHD 10.000 into three equal installments cannot be perfectly solved. The chosen values total exactly 10.000. Halving any of these breaks the reconciliation.