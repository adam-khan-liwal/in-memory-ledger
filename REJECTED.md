# Rejected Acceptance Criteria

The following acceptance criteria provided in the requirements were identified as incorrect and have been rejected:

**1. "E7 causes exactly one overdraft fee to be assessed, on Day 2."**
* **Reason for Rejection:** The ledger is append-only and processes E7 on Day 5 (backdated to Day 2). Because E7 retroactively reduces the balance by 620.00, it actually pushes the end-of-day balances for multiple days (Day 2 and Day 4) into the negative. Furthermore, since the ledger only becomes aware of this backdated debit on Day 5, the overdraft fee(s) triggered by past negative balances are assessed and booked on the processing day (Day 5), not retroactively on Day 2.

**2. "Any settlement referencing an authorization ID not present in the ledger must be rejected and the funds must not leave the account."**
* **Reason for Rejection:** This violates standard payment network principles (like "force post" transactions). A ledger must accept a settlement (like E6 / Auth-Z) even if a matching authorization hold was not found, dropped, or expired, as clearing instructions dictate actual movement of funds that the bank is legally obligated to settle.

**3. "After E9, all balances and fees return to their pre-E7 values."**
* **Reason for Rejection:** The ledger is strictly append-only ("No event record is ever mutated or deleted"). E9 reverses the principal amount of E7 by posting a new compensating transaction. However, the overdraft fees assessed on Day 5 due to E7 are standalone fee events; they do not automatically disappear. They would require explicit fee-reversal events to be negated. 

**4. "The three BHD instalments in E10 must each be BHD 3.334."**
* **Reason for Rejection:** BHD 3.334 × 3 equals BHD 10.002, which is greater than the total intended credit of BHD 10.000. To prevent generating money out of thin air, an exact distribution of BHD 10.000 over three instalments in a 3-decimal currency must be split as BHD 3.334, BHD 3.333, and BHD 3.333.

**5. "If the rounded daily interest accruals do not sum to the capitalized total, the remainder is discarded."**
* **Reason for Rejection:** Discarding remainders violates basic accounting reconciliation principles. The rules specifically state: *"The rounded daily accruals must sum exactly to the capitalized total."* Instead of discarding the difference, any rounding penny/fil differences should be adjusted into the final accrual (or the capitalized total forced to match the sum of the rounded daily accruals) so the ledger remains fully balanced.