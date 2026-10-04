# Criteria Evaluation Report (REJECTED.md)

This document evaluates the complete set of operational criteria for the core banking ledger, identifying which claims are correct and which are rejected, complete with accounting and system reasoning.

---

### Criterion 1
* **Statement:** The Day 2 closing ledger balance, evaluated at end of Day 5 and before any fee is assessed, is AED −370.00.
* **Status:** **Correct**
* **Reason:** Summing all value-dated entries up to Day 2 processed on or before Day 5 gives: E1 (+1200.00) + E2 (−950.00) + E7 (−620.00, value-dated Day 2) = −370.00. Holds (E3) and future-dated transactions do not affect this historical ledger calculation.
=====================================================================================================
          LEDGER BALANCES BEFORE FEES PER ACCOUNT PER DAY AS OF PROCESSING DATE 2026-01-05               
=====================================================================================================
Account ID                | 2026-01-01   | 2026-01-02   | 2026-01-03   | 2026-01-04  
-----------------------------------------------------------------------------------------------------
ACC-001                   | AED 250.00   | AED -370.00  | AED 30.00    | AED -335.00 
ACC-002                   | BHD 0.000    | BHD 0.000    | BHD 0.000    | BHD 0.000   

---

### Criterion 2
* **Statement:** E7 causes exactly one overdraft fee to be assessed, on Day 2.
* **Status:** **Incorrect**
* **Reason:** E7 makes the value-dated balance negative across multiple days. Overdraft fee is issued on Day 2, Day 4 as well as Day 5. However all these are fees were reversed after E9.

Output from program after E7:

```
[RECALC] Inserted Fee Delta: AED 25.00 for Value Date 2026-01-02 (Expected: AED 25.00, Recorded in GL: AED 0.00)
[RECALC] Inserted Accrual Delta: AED -0.10 for Value Date 2026-01-02 (Balance: AED -395.00, Expected: AED 0.00, Recorded in GL: AED 0.10)
[RECALC] Inserted Accrual Delta: AED -0.26 for Value Date 2026-01-03 (Balance: AED 5.00, Expected: AED 0.00, Recorded in GL: AED 0.26)
[RECALC] Inserted Fee Delta: AED 25.00 for Value Date 2026-01-04 (Expected: AED 25.00, Recorded in GL: AED 0.00)
[RECALC] Inserted Accrual Delta: AED -0.11 for Value Date 2026-01-04 (Balance: AED -385.00, Expected: AED 0.00, Recorded in GL: AED 0.11)
>>> End of Day 2026-01-05 Processing <<<
Assessed Overdraft Fee: AED 25.00 (Triggered by Value Date 2026-01-05 balance)
    | --- LEDGER SNAPSHOT ---
    | ACC-001 - Ledger: AED -410.00 | Available: AED -410.00
    | ACC-002 - Ledger: BHD 0.000 | Available: BHD 0.000
    | [Internal GL] GL-ASSET-CASH-AED - Balance: AED -335.00
    | [Internal GL] GL-INT-PAYABLE-ACC-001 - Balance: AED 0.10
    | [Internal GL] GL-INT-EXPENSE-ACC-001 - Balance: AED 0.10
    | [Internal GL] GL-ASSET-CASH-BHD - Balance: BHD 0.000
    | [Internal GL] GL-INT-PAYABLE-ACC-002 - Balance: BHD 0.000
    | [Internal GL] GL-INT-EXPENSE-ACC-002 - Balance: BHD 0.000
    | -----------------------
```

---

### Criterion 3
* **Statement:** The Day 4 settlement of Auth-A must be accepted.
* **Status:** **Correct**
* **Reason:** Event E5 represents a valid settlement for AED 185.00 against an active authorization hold (Auth-A of AED 200.00), which correctly releases the hold and posts the debit to the ledger.

---

### Criterion 4
* **Statement:** Any settlement referencing an authorization ID not present in the ledger must be rejected and the funds must not leave the account.
* **Status:** **Incorrect**
* **Reason:** This violates payment network clearing standards. Settlements are binding financial instructions from the clearing network. If an authorization hold is missing (due to network drops or expiration), the ledger must permit "force-posting" to avoid catastrophic clearing scheme reconciliation failures.

---

### Criterion 5
* **Statement:** If Auth-B is approved, its hold reduces available balance but not ledger balance.
* **Status:** **Correct**
* **Reason:** Authorizations are administrative liquidity locks, not monetary movements. They freeze spending power (available balance) but leave the legal asset/liability state (ledger balance) unchanged until actual settlement occurs.

---

### Criterion 6
* **Statement:** After E9, all balances and fees return to their pre-E7 values.
* **Status:** **Correct**
* **Reason:** The Acc-001 and fees account returned to their original balances. 

Pre E7 Ledger Snapshot:
```
| --- LEDGER SNAPSHOT ---
| ACC-001 - Ledger: AED 285.00 | Available: AED 285.00
| ACC-002 - Ledger: BHD 0.000 | Available: BHD 0.000
| [Internal GL] GL-ASSET-CASH-AED - Balance: AED 285.00
| [Internal GL] GL-INT-PAYABLE-ACC-001 - Balance: AED 0.57
| [Internal GL] GL-INT-EXPENSE-ACC-001 - Balance: AED 0.57
| [Internal GL] GL-ASSET-CASH-BHD - Balance: BHD 0.000
| [Internal GL] GL-INT-PAYABLE-ACC-002 - Balance: BHD 0.000
| [Internal GL] GL-INT-EXPENSE-ACC-002 - Balance: BHD 0.000
| -----------------------
```

Post E9 Ledger Snapshot (Ignore the BHD entry, as the recaclution of the fees and accruals is done at end of day therefore E10 is also included in this snapshot)
```
| --- LEDGER SNAPSHOT ---
| ACC-001 - Ledger: AED 285.00 | Available: AED 285.00
| ACC-002 - Ledger: BHD 10.000 | Available: BHD 10.000
| [Internal GL] GL-ASSET-CASH-AED - Balance: AED 285.00
| [Internal GL] GL-INT-PAYABLE-ACC-001 - Balance: AED 0.68
| [Internal GL] GL-INT-EXPENSE-ACC-001 - Balance: AED 0.68
| [Internal GL] GL-ASSET-CASH-BHD - Balance: BHD 10.000
| [Internal GL] GL-INT-PAYABLE-ACC-002 - Balance: BHD 0.004
| [Internal GL] GL-INT-EXPENSE-ACC-002 - Balance: BHD 0.004
| -----------------------
```

### Criterion 7
* **Statement:** The three BHD instalments in E10 must each be BHD 3.334.
* **Status:** **Incorrect**
* **Reason:** Mathematical impossibility: $3.334 \times 3 = 10.002$, which manufactures BHD 0.002 out of thin air and violates double-entry conservation. The correct distribution to total BHD 10.000 across three parts is 3.334, 3.333, and 3.333.

---

### Criterion 8
* **Statement:** If the rounded daily interest accruals do not sum to the capitalized total, the remainder is discarded.
* **Status:** **Incorrect**
* **Reason:** Discarding floating-point or rounding remainders causes permanent discrepancies in general ledger accounting. Rounded daily accruals must reconcile exactly to the capitalized total via precise adjustment tracking rather than dropping fractions of currency units.