# Development Worklog

* **14:00** - Used AI to generate structures and basic functions of ledger.
* **15:46** - Most AI responses would replay the full stream of events and then calculate the overdraft fee for each day, which results in no overdraft. But my assumption is, the event stream is real time and the overdraft fee need to be checked each day for all accounts. On day 5, the AED account goes negative as 620 is debited and therefore an overdraft fee is charged.
* **16:00** - Added ledger balances by processing day to confirm criteria no. 1.
* **16:30** - Noted findings and cross checked with all the criteria and filled the required .md files REJECTED.md NUMBERS.md, AMBIGUITIES.md, WORKLOG.md
