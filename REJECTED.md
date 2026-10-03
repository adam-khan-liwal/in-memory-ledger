# Criteria Evaluation Report (REJECTED.md)

This document evaluates the complete set of operational criteria for the core banking ledger, identifying which claims are correct and which are rejected, complete with accounting and system reasoning.

---

### Criterion 1
* **Statement:** The Day 2 closing ledger balance, evaluated at end of Day 5 and before any fee is assessed, is AED −370.00.
* **Status:** **Correct**
* **Reason:** Summing all value-dated entries up to Day 2 processed on or before Day 5 gives: E1 (+1200.00) + E2 (−950.00) + E7 (−620.00, value-dated Day 2) = −370.00. Holds (E3) and future-dated transactions do not affect this historical ledger calculation.
=====================================================================================================
                   LEDGER BALANCES PER ACCOUNT PER DAY AS OF PROCESSING DATE 2026-01-05               
=====================================================================================================
Account ID                | 2026-01-01   | 2026-01-02   | 2026-01-03   | 2026-01-04  
-----------------------------------------------------------------------------------------------------
ACC-001                   | AED 250.00   | AED -370.00  | AED 30.00    | AED -335.00 
ACC-002                   | BHD 0.000    | BHD 0.000    | BHD 0.000    | BHD 0.000   
GL-ASSET-CASH-AED         | AED 250.00   | AED -370.00  | AED 30.00    | AED -335.00 
GL-ASSET-CASH-BHD         | BHD 0.000    | BHD 0.000    | BHD 0.000    | BHD 0.000   
GL-INT-PAYABLE-ACC-001    | AED 0.10     | AED 0.20     | AED 0.46     | AED 0.57    
GL-INT-EXPENSE-ACC-001    | AED 0.10     | AED 0.20     | AED 0.46     | AED 0.57    
GL-INT-PAYABLE-ACC-002    | BHD 0.000    | BHD 0.000    | BHD 0.000    | BHD 0.000   
GL-INT-EXPENSE-ACC-002    | BHD 0.000    | BHD 0.000    | BHD 0.000    | BHD 0.000   
=====================================================================================================

---

### Criterion 2
* **Statement:** E7 causes exactly one overdraft fee to be assessed, on Day 2.
* **Status:** **Incorrect**
* **Reason:** E7 makes the value-dated balance negative across multiple days. But we are not assessing past days, we are in Day 5 and the overdraft fee is issued on Day 5. It is not specified that we can retroactively assess each day if the value date is changed.

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
* **Status:** **Incorrect**
* **Reason:** Because of E7 AED 620 debit, the account balance went negative and an overdraft fee of AED 25 was charged on Day 5. 
Pre E7 Ledger Snapshot:
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

Post E9 Ledger Snapshot
| --- LEDGER SNAPSHOT ---
| ACC-001 - Ledger: AED 260.00 | Available: AED 260.00
| ACC-002 - Ledger: BHD 0.000 | Available: BHD 0.000
| [Internal GL] GL-ASSET-CASH-AED - Balance: AED 260.00
| [Internal GL] GL-INT-PAYABLE-ACC-001 - Balance: AED 0.57
| [Internal GL] GL-INT-EXPENSE-ACC-001 - Balance: AED 0.57
| [Internal GL] GL-ASSET-CASH-BHD - Balance: BHD 0.000
| [Internal GL] GL-INT-PAYABLE-ACC-002 - Balance: BHD 0.000
| [Internal GL] GL-INT-EXPENSE-ACC-002 - Balance: BHD 0.000
| -----------------------
---

### Criterion 7
* **Statement:** The three BHD instalments in E10 must each be BHD 3.334.
* **Status:** **Incorrect**
* **Reason:** Mathematical impossibility: $3.334 \times 3 = 10.002$, which manufactures BHD 0.002 out of thin air and violates double-entry conservation. The correct distribution to total BHD 10.000 across three parts is 3.334, 3.333, and 3.333.

---

### Criterion 8
* **Statement:** If the rounded daily interest accruals do not sum to the capitalized total, the remainder is discarded.
* **Status:** **Incorrect**
* **Reason:** Discarding floating-point or rounding remainders causes permanent discrepancies in general ledger accounting. Rounded daily accruals must reconcile exactly to the capitalized total via precise adjustment tracking rather than dropping fractions of currency units.