# Ambiguities Resolved
1. **Incomplete ledger accounts provided:**
   * *Ambiguity:* Only two accounts provided AED and BHD. For a proper ledger system, we need balancing accounts for double entry accoutning.
   * *Resolution:* I added seperate accounts for Accrual interests (along with their expense accounts). And also added two Asset accounts for the AED and BHD accounts provided.
2. **Backdated Debits & Overdraft Fees:** 
   * *Ambiguity:* E7 is processed on Day 5 but backdated to Day 2. 
   * *Resolution:* A core banking system must penalize *every* day the account was overdrawn. The rules state: Overdraft fee is "assessed once per day per account when that day's closing ledger balance (all entries with value_date ≤ that day) is negative". This suggests we should retroactively recalculate the fes for all the days since a backdated value date.
3. **Settlement without Authorization (E6):**
   * *Ambiguity:* E6 has no preceding hold. Should the ledger reject it?
   * *Resolution:* No. Settlements are legal clearing instructions from the network. I implemented this as a "force post" which immediately debits the ledger balance regardless of whether a hold existed.
4. **Reversal Operations:**
   * *Ambiguity:* E9 reverses E7. Does a reversal blindly add money, or does it depend on the target event?
   * *Resolution:* Made dynamic. The logic fetches the `TargetEventID`. If the target was a Debit/Fee, the reversal credits the account. If the target was a Credit, the reversal debits it.
5. **Late-Arriving Events in Stream:**
   * *Ambiguity:* E10 has a Day 5 processing date but sits sequentially after E9 (Day 6). 
   * *Resolution:* Grouping events upfront hides stream reality. I resolved this by building a strict top-down stream reader. If an event's `ProcessingDay` is earlier than the currently open day, the ledger traps it, rewrites its `ProcessingDay` to today, but keeps its `ValueDate` intact for historical calculations.