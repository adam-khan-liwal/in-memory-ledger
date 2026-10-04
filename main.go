package main

import (
	"fmt"
	"math"
	"time"
)

type EventType string

const (
	Credit EventType = "CREDIT"
	Debit  EventType = "DEBIT"
)

const OVERDRAFT_FEE = 25
const INTEREST_RATE = 0.0004

type Event struct {
	ID            string
	ProcessingDay time.Time
	ValueDate     time.Time
	Type          EventType
	AccountID     string
	Amount        int64
	Currency      string
	RefID         string
	TargetEventID string
}

type Account struct {
	ID            string
	Currency      string
	Precision     int
	Multiplier    int64
	IsInternal    bool
	NormalBalance string // "DEBIT" or "CREDIT"
}

type Ledger struct {
	StartDate    time.Time
	Accounts     map[string]*Account
	Events       []*Event
	ActiveHolds  map[string]map[string]int64
	FeesAssessed map[string]map[time.Time]bool
}

func NewLedger(startDate time.Time) *Ledger {
	return &Ledger{
		StartDate:    startDate,
		Accounts:     make(map[string]*Account),
		Events:       make([]*Event, 0),
		ActiveHolds:  make(map[string]map[string]int64),
		FeesAssessed: make(map[string]map[time.Time]bool),
	}
}

func (l *Ledger) AddAccount(id, currency string, precision int, isInternal bool, normalBalance string) {
	mult := int64(math.Pow10(precision))
	l.Accounts[id] = &Account{
		ID:            id,
		Currency:      currency,
		Precision:     precision,
		Multiplier:    mult,
		IsInternal:    isInternal,
		NormalBalance: normalBalance,
	}
	if !isInternal {
		l.ActiveHolds[id] = make(map[string]int64)
	}
	l.FeesAssessed[id] = make(map[time.Time]bool)
}

func (l *Ledger) formatAmount(accId string, amount int64) string {
	acc := l.Accounts[accId]
	format := fmt.Sprintf("%%.%df", acc.Precision)
	return fmt.Sprintf("%s "+format, acc.Currency, float64(amount)/float64(acc.Multiplier))
}

func onOrBefore(t1, t2 time.Time) bool {
	return t1.Before(t2) || t1.Equal(t2)
}

func (l *Ledger) LedgerBalanceAt(accId string, asOf time.Time) int64 {
	return l.LedgerBalanceAt1(accId, asOf, nil)
}

// LedgerBalanceAt calculates balance based on normal balance accounting rules:
// - Asset / Expense (DEBIT normal): Debits - Credits
// - Liability / Equity / Revenue (CREDIT normal): Credits - Debits
func (l *Ledger) LedgerBalanceAt1(accId string, asOf time.Time, asOfProcessingDate *time.Time) int64 {
	acc := l.Accounts[accId]
	var debits int64 = 0
	var credits int64 = 0
	for _, e := range l.Events {
		if e.AccountID == accId && onOrBefore(e.ValueDate, asOf) {
			// fmt.Println(asOf)
			// fmt.Println(acc.ID, e.Type, e.Amount)
			if asOfProcessingDate != nil {
				if !onOrBefore(e.ProcessingDay, *asOfProcessingDate) {
					break
				}
			}
			switch e.Type {
			case Debit:
				debits += e.Amount
			case Credit:
				credits += e.Amount
			}
		}
	}
	if acc.NormalBalance == "DEBIT" {
		return debits - credits
	}
	return credits - debits
}

func GetEvent(id string, events []*Event) *Event {
	for _, target := range events {
		if target.ID == id {
			return target
		}
	}
	return nil
}

func (l *Ledger) AvailableBalanceAt(accId string, asOf time.Time) int64 {
	balance := l.LedgerBalanceAt(accId, asOf)
	var activeHolds int64 = 0
	for _, holdAmount := range l.ActiveHolds[accId] {
		activeHolds += holdAmount
	}
	return balance - activeHolds
}

func (l *Ledger) postEntry(id string, processingDay, valueDate time.Time, t EventType, accID string, amount int64, currency string) {
	l.Events = append(l.Events, &Event{
		ID:            id,
		ProcessingDay: processingDay,
		ValueDate:     valueDate,
		Type:          t,
		AccountID:     accID,
		Amount:        amount,
		Currency:      currency,
	})
}

func (l *Ledger) ProcessEvent(e *Event) error {
	acc := l.Accounts[e.AccountID]
	assetAccID := "GL-ASSET-CASH-" + e.Currency

	switch e.Type {
	case "AUTHORIZATION":
		avail := l.AvailableBalanceAt(acc.ID, e.ProcessingDay)
		if avail-e.Amount < 0 {
			return fmt.Errorf("authorization declined: insufficient available balance")
		}
		l.ActiveHolds[acc.ID][e.RefID] = e.Amount

	case "SETTLEMENT":
		delete(l.ActiveHolds[acc.ID], e.RefID)
		l.postEntry(e.ID+"-DR", e.ProcessingDay, e.ValueDate, Debit, acc.ID, e.Amount, e.Currency)
		l.postEntry(e.ID+"-CR", e.ProcessingDay, e.ValueDate, Credit, assetAccID, e.Amount, e.Currency)

	case "CREDIT":
		l.postEntry(e.ID+"-CR", e.ProcessingDay, e.ValueDate, Credit, acc.ID, e.Amount, e.Currency)
		l.postEntry(e.ID+"-DR", e.ProcessingDay, e.ValueDate, Debit, assetAccID, e.Amount, e.Currency)

	case "DEBIT":
		l.postEntry(e.ID+"-DR", e.ProcessingDay, e.ValueDate, Debit, acc.ID, e.Amount, e.Currency)
		l.postEntry(e.ID+"-CR", e.ProcessingDay, e.ValueDate, Credit, assetAccID, e.Amount, e.Currency)

	case "FEE":
		l.postEntry(e.ID+"-DR", e.ProcessingDay, e.ValueDate, Debit, acc.ID, e.Amount, e.Currency)
		l.postEntry(e.ID+"-CR", e.ProcessingDay, e.ValueDate, Credit, "GL-FEE-INCOME-"+e.Currency, e.Amount, e.Currency)

	case "ACCRUAL":
		expenseGL := "GL-INT-EXPENSE-" + acc.ID
		payableGL := "GL-INT-PAYABLE-" + acc.ID
		l.postEntry(e.ID+"-EXP-DR", e.ProcessingDay, e.ValueDate, Debit, expenseGL, e.Amount, e.Currency)
		l.postEntry(e.ID+"-PAY-CR", e.ProcessingDay, e.ValueDate, Credit, payableGL, e.Amount, e.Currency)

	case "INTEREST":
		payableGL := "GL-INT-PAYABLE-" + acc.ID
		l.postEntry(e.ID+"-PAY-DR", e.ProcessingDay, e.ValueDate, Debit, payableGL, e.Amount, e.Currency)
		l.postEntry(e.ID+"-ACC-CR", e.ProcessingDay, e.ValueDate, Credit, acc.ID, e.Amount, e.Currency)

	case "REVERSAL":
		baseID := e.TargetEventID
		drLeg := GetEvent(baseID+"-DR", l.Events)
		crLeg := GetEvent(baseID+"-CR", l.Events)
		if drLeg != nil {
			revType := Credit
			if drLeg.Type == Credit {
				revType = Debit
			}
			l.postEntry(e.ID+"-"+drLeg.AccountID+"-REV", e.ProcessingDay, e.ValueDate, revType, drLeg.AccountID, drLeg.Amount, drLeg.Currency)
		}
		if crLeg != nil {
			revType := Credit
			if crLeg.Type == Credit {
				revType = Debit
			}
			l.postEntry(e.ID+"-"+crLeg.AccountID+"-REV", e.ProcessingDay, e.ValueDate, revType, crLeg.AccountID, crLeg.Amount, crLeg.Currency)
		}
	}
	return nil
}

func (l *Ledger) PrintLedgerState(asOf time.Time) {
	fmt.Println("    | --- LEDGER SNAPSHOT ---")

	accIDs := []string{
		"ACC-001", "ACC-002",
		"GL-ASSET-CASH-AED", "GL-INT-PAYABLE-ACC-001", "GL-INT-EXPENSE-ACC-001",
		"GL-ASSET-CASH-BHD", "GL-INT-PAYABLE-ACC-002", "GL-INT-EXPENSE-ACC-002",
	}
	for _, accId := range accIDs {
		acc := l.Accounts[accId]
		if acc == nil {
			continue
		}
		ledgerBal := l.LedgerBalanceAt(accId, asOf)

		if acc.IsInternal {
			fmt.Printf("    | [Internal GL] %s - Balance: %s\n", accId, l.formatAmount(accId, ledgerBal))
		} else {
			availBal := l.AvailableBalanceAt(accId, asOf)
			holdsStr := ""
			if len(l.ActiveHolds[accId]) > 0 {
				holdsStr = " | Active Holds: "
				for authID, amt := range l.ActiveHolds[accId] {
					holdsStr += fmt.Sprintf("[%s: %s] ", authID, l.formatAmount(accId, amt))
				}
			}
			fmt.Printf("    | %s - Ledger: %s | Available: %s%s\n",
				accId, l.formatAmount(accId, ledgerBal), l.formatAmount(accId, availBal), holdsStr)
		}
	}
	fmt.Println("    | -----------------------")
}
func (l *Ledger) PrintTrialBalance(asOf time.Time) {
	fmt.Println("\n==========================================")
	fmt.Println("             TRIAL BALANCE                ")
	fmt.Println("==========================================")
	fmt.Printf("%-30s | %-15s | %-15s\n", "Account ID", "Debit", "Credit")
	fmt.Println("------------------------------------------------------------------")

	var totalDebitsAED, totalCreditsAED int64
	var totalDebitsBHD, totalCreditsBHD int64

	// Sort or iterate through all accounts
	for _, accId := range []string{
		"ACC-001", "ACC-002",
		"GL-ASSET-CASH-AED", "GL-ASSET-CASH-BHD",
		"GL-FEE-INCOME-AED", "GL-FEE-INCOME-BHD",
		"GL-INT-PAYABLE-ACC-001", "GL-INT-EXPENSE-ACC-001",
		"GL-INT-PAYABLE-ACC-002", "GL-INT-EXPENSE-ACC-002",
	} {
		acc := l.Accounts[accId]
		if acc == nil {
			continue
		}

		ledgerBal := l.LedgerBalanceAt(accId, asOf)
		if ledgerBal == 0 {
			continue
		}

		var debitStr, creditStr string
		if acc.NormalBalance == "DEBIT" {
			if acc.Currency == "AED" {
				totalDebitsAED += ledgerBal
			} else {
				totalDebitsBHD += ledgerBal
			}
			debitStr = l.formatAmount(accId, ledgerBal)
			creditStr = "-"
		} else {
			if acc.Currency == "AED" {
				totalCreditsAED += ledgerBal
			} else {
				totalCreditsBHD += ledgerBal
			}
			debitStr = "-"
			creditStr = l.formatAmount(accId, ledgerBal)
		}

		fmt.Printf("%-30s | %-15s | %-15s\n", accId, debitStr, creditStr)
	}

	fmt.Println("------------------------------------------------------------------")
	fmt.Println("--- AED Totals ---")
	fmt.Printf("Total Debits:   AED %.2f\n", float64(totalDebitsAED)/100.0)
	fmt.Printf("Total Credits:  AED %.2f\n", float64(totalCreditsAED)/100.0)

	fmt.Println("--- BHD Totals ---")
	fmt.Printf("Total Debits:   BHD %.3f\n", float64(totalDebitsBHD)/1000.0)
	fmt.Printf("Total Credits:  BHD %.3f\n", float64(totalCreditsBHD)/1000.0)
	fmt.Println("==========================================")
}
func (l *Ledger) EndOfDay(currentDay time.Time, isLastDay bool) {
	fmt.Printf("\n>>> End of Day %s Processing <<<\n", currentDay.Format("2006-01-02"))

	for _, acc := range l.Accounts {
		if acc.IsInternal {
			continue
		}

		// 1. Overdraft fee evaluations (AED only)
		balance := l.LedgerBalanceAt(acc.ID, currentDay)
		if balance < 0 && !l.FeesAssessed[acc.ID][currentDay] {
			feeAmount := int64(OVERDRAFT_FEE * acc.Multiplier)
			if acc.Currency == "AED" {
				feeEvent := &Event{
					ID:            fmt.Sprintf("FEE-%s-D%s", acc.ID, currentDay.Format("20060102")),
					ProcessingDay: currentDay,
					ValueDate:     currentDay,
					Type:          "FEE",
					AccountID:     acc.ID,
					Amount:        feeAmount,
					Currency:      acc.Currency,
				}
				l.ProcessEvent(feeEvent)
				l.FeesAssessed[acc.ID][currentDay] = true
				fmt.Printf("Assessed Overdraft Fee: %s (Triggered by Value Date %s balance)\n",
					l.formatAmount(acc.ID, feeAmount), currentDay.Format("2006-01-02"))
				l.PrintLedgerState(currentDay)
			}
		}

		// 2. Daily Interest Accrual
		eodBalance := l.LedgerBalanceAt(acc.ID, currentDay)
		if eodBalance > 0 {
			exactAccrual := float64(eodBalance) * INTEREST_RATE
			roundedAccrual := int64(math.Round(exactAccrual))
			if roundedAccrual > 0 {
				payableGL := "GL-INT-PAYABLE-" + acc.ID
				expenseGL := "GL-INT-EXPENSE-" + acc.ID

				accrualEvent := &Event{
					ID:            fmt.Sprintf("ACCRUAL-%s-%s", acc.ID, currentDay.Format("20060102")),
					ProcessingDay: currentDay,
					ValueDate:     currentDay,
					Type:          "ACCRUAL",
					AccountID:     acc.ID,
					Amount:        roundedAccrual,
					Currency:      acc.Currency,
				}
				l.ProcessEvent(accrualEvent)

				fmt.Printf("Accrued Interest: %s (Expense Dr: %s, Payable Cr: %s)\n",
					l.formatAmount(acc.ID, roundedAccrual), expenseGL, payableGL)
				l.PrintLedgerState(currentDay)
			}
		}

		// 3. Capitalize Interest on Final Day
		if isLastDay {
			payableGL := "GL-INT-PAYABLE-" + acc.ID
			totalAccrued := l.LedgerBalanceAt(payableGL, currentDay)
			if totalAccrued > 0 {
				intEvent := &Event{
					ID:            fmt.Sprintf("INT-%s", acc.ID),
					ProcessingDay: currentDay,
					ValueDate:     currentDay,
					Type:          "INTEREST",
					AccountID:     acc.ID,
					Amount:        totalAccrued,
					Currency:      acc.Currency,
				}
				l.ProcessEvent(intEvent)
				fmt.Printf("Capitalized Interest: %s transferred from %s to %s\n", l.formatAmount(acc.ID, totalAccrued), payableGL, acc.ID)
				l.PrintLedgerState(currentDay)
			}
		}
	}
}

func main() {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	days := []time.Time{
		base,
		base.AddDate(0, 0, 1),
		base.AddDate(0, 0, 2),
		base.AddDate(0, 0, 3),
		base.AddDate(0, 0, 4),
		base.AddDate(0, 0, 5),
	}

	l := NewLedger(days[0])

	l.AddAccount("ACC-001", "AED", 2, false, "CREDIT")
	l.AddAccount("ACC-002", "BHD", 3, false, "CREDIT")

	l.AddAccount("GL-ASSET-CASH-AED", "AED", 2, true, "DEBIT")
	l.AddAccount("GL-ASSET-CASH-BHD", "BHD", 3, true, "DEBIT")

	l.AddAccount("GL-FEE-INCOME-AED", "AED", 2, true, "CREDIT")
	l.AddAccount("GL-FEE-INCOME-BHD", "BHD", 3, true, "CREDIT")

	l.AddAccount("GL-INT-PAYABLE-ACC-001", "AED", 2, true, "CREDIT")
	l.AddAccount("GL-INT-EXPENSE-ACC-001", "AED", 2, true, "DEBIT")
	l.AddAccount("GL-INT-PAYABLE-ACC-002", "BHD", 3, true, "CREDIT")
	l.AddAccount("GL-INT-EXPENSE-ACC-002", "BHD", 3, true, "DEBIT")

	stream := []*Event{
		{ID: "E1", ProcessingDay: days[0], ValueDate: days[0], Type: "CREDIT", AccountID: "ACC-001", Amount: 120000, Currency: "AED"},
		{ID: "E2", ProcessingDay: days[0], ValueDate: days[0], Type: "DEBIT", AccountID: "ACC-001", Amount: 95000, Currency: "AED"},
		{ID: "E3", ProcessingDay: days[1], ValueDate: days[1], Type: "AUTHORIZATION", AccountID: "ACC-001", Amount: 20000, Currency: "AED", RefID: "Auth-A"},
		{ID: "E4", ProcessingDay: days[2], ValueDate: days[2], Type: "CREDIT", AccountID: "ACC-001", Amount: 40000, Currency: "AED"},
		{ID: "E5", ProcessingDay: days[3], ValueDate: days[3], Type: "SETTLEMENT", AccountID: "ACC-001", Amount: 18500, Currency: "AED", RefID: "Auth-A"},
		{ID: "E6", ProcessingDay: days[3], ValueDate: days[3], Type: "SETTLEMENT", AccountID: "ACC-001", Amount: 18000, Currency: "AED", RefID: "Auth-Z"},
		{ID: "E7", ProcessingDay: days[4], ValueDate: days[1], Type: "DEBIT", AccountID: "ACC-001", Amount: 62000, Currency: "AED"},
		{ID: "E8", ProcessingDay: days[4], ValueDate: days[4], Type: "AUTHORIZATION", AccountID: "ACC-001", Amount: 9000, Currency: "AED", RefID: "Auth-B"},
		{ID: "E9", ProcessingDay: days[5], ValueDate: days[1], Type: "REVERSAL", AccountID: "ACC-001", TargetEventID: "E7", Currency: "AED"},
		{ID: "E10-1", ProcessingDay: days[4], ValueDate: days[4], Type: "CREDIT", AccountID: "ACC-002", Amount: 3334, Currency: "BHD"},
		{ID: "E10-2", ProcessingDay: days[4], ValueDate: days[4], Type: "CREDIT", AccountID: "ACC-002", Amount: 3333, Currency: "BHD"},
		{ID: "E10-3", ProcessingDay: days[4], ValueDate: days[4], Type: "CREDIT", AccountID: "ACC-002", Amount: 3333, Currency: "BHD"},
	}

	currentOpenDay := days[0]
	recalculate := false
	recalculate_from := currentOpenDay

	fmt.Printf("\n==========================================\n")
	fmt.Printf(" STARTING DAY: %s \n", currentOpenDay.Format("2006-01-02"))
	fmt.Printf("==========================================\n")

	for _, e := range stream {
		// if e.ProcessingDay.Before(currentOpenDay) {
		// 	e.ProcessingDay = currentOpenDay
		// }

		for e.ProcessingDay.After(currentOpenDay) {

			fmt.Println("recalculate", recalculate, "from", recalculate_from)
			if recalculate {
				l.RecalculateFrom(recalculate_from, currentOpenDay)
			}

			isLastDay := currentOpenDay.Equal(days[len(days)-1])
			l.EndOfDay(currentOpenDay, isLastDay)
			currentOpenDay = currentOpenDay.AddDate(0, 0, 1)

			if onOrBefore(currentOpenDay, days[len(days)-1]) {
				fmt.Printf("\n==========================================\n")
				fmt.Printf(" STARTING DAY: %s \n", currentOpenDay.Format("2006-01-02"))
				fmt.Printf("==========================================\n")

				recalculate = false
				recalculate_from = currentOpenDay
			}
		}

		err := l.ProcessEvent(e)

		if err != nil {
			fmt.Printf("\n[X] EVENT FAILED: %s (%v)\n", e.ID, err)
		} else {
			fmt.Printf("\n[+] PROCESSED EVENT: %s (%s %s)\n", e.ID, e.Type, l.formatAmount(e.AccountID, e.Amount))

			if e.ValueDate.Before(currentOpenDay) {
				fmt.Println("BACKDATED ENTRY, WILL RECALCULATE AT END OF DAY")
				recalculate = true
				if e.ValueDate.Before(recalculate_from) {
					recalculate_from = e.ValueDate
				}
			}

			l.PrintLedgerState(currentOpenDay)

		}
	}

	for onOrBefore(currentOpenDay, days[len(days)-1]) {
		if recalculate {
			l.RecalculateFrom(recalculate_from, currentOpenDay)
		}

		isLastDay := currentOpenDay.Equal(days[len(days)-1])
		l.EndOfDay(currentOpenDay, isLastDay)
		if !isLastDay {
			currentOpenDay = currentOpenDay.AddDate(0, 0, 1)
		} else {
			break
		}
	}

	l.PrintTrialBalance(days[len(days)-1])

	// l.PrintDailyBalances(days[:4], days[4])
}

func (l *Ledger) PrintDailyBalances(days []time.Time, asOfProcessingDate time.Time) {
	fmt.Println("\n=====================================================================================================")
	fmt.Printf("                   LEDGER BALANCES PER ACCOUNT PER DAY AS OF PROCESSING DATE %s               \n", asOfProcessingDate.Format("2006-01-02"))
	fmt.Println("=====================================================================================================")

	accIDs := []string{
		"ACC-001", "ACC-002",
		"GL-ASSET-CASH-AED", "GL-ASSET-CASH-BHD",
		"GL-INT-PAYABLE-ACC-001", "GL-INT-EXPENSE-ACC-001",
		"GL-INT-PAYABLE-ACC-002", "GL-INT-EXPENSE-ACC-002",
	}

	// Print Header
	header := fmt.Sprintf("%-25s", "Account ID")
	for _, d := range days {
		header += fmt.Sprintf(" | %-12s", d.Format("2006-01-02"))
	}
	fmt.Println(header)
	fmt.Println("-----------------------------------------------------------------------------------------------------")

	// Print Row per Account
	for _, accId := range accIDs {
		row := fmt.Sprintf("%-25s", accId)
		for _, d := range days {
			bal := l.LedgerBalanceAt1(accId, d, &asOfProcessingDate)
			row += fmt.Sprintf(" | %-12s", l.formatAmount(accId, bal))
		}
		fmt.Println(row)
	}
	fmt.Println("=====================================================================================================")
}

// RecalculateFrom reruns fees and interest accruals from backDate up to currentOpenDay,
// checking the GL accounts (GL-FEE-INCOME and GL-INT-PAYABLE) for existing entries on each value date,
// and inserts delta adjustment entries if discrepancies are found.
func (l *Ledger) RecalculateFrom(backDate, currentOpenDay time.Time) {
	fmt.Printf("\n--- RECALCULATING FEES & ACCRUALS FROM %s TO %s ---\n",
		backDate.Format("2006-01-02"), currentOpenDay.Format("2006-01-02"))

	d := backDate
	for d.Before(currentOpenDay) {
		for _, acc := range l.Accounts {
			if acc.IsInternal {
				continue
			}

			// Define corresponding GL account IDs
			feeIncomeGL := "GL-FEE-INCOME-" + acc.Currency
			payableGL := "GL-INT-PAYABLE-" + acc.ID

			// 1. Check existing recorded fee via GL-FEE-INCOME entries on value date 'd'
			var recordedFee int64 = 0
			for _, ev := range l.Events {
				if ev.AccountID == feeIncomeGL && ev.ValueDate.Equal(d) {
					switch ev.Type {
					case Credit:
						recordedFee += ev.Amount
					case Debit:
						recordedFee -= ev.Amount
					}
				}
			}

			// 2. Check existing recorded accrual via GL-INT-PAYABLE entries on value date 'd'
			var recordedAccrual int64 = 0
			for _, ev := range l.Events {
				if ev.AccountID == payableGL && ev.ValueDate.Equal(d) {
					switch ev.Type {
					case Credit:
						recordedAccrual += ev.Amount
					case Debit:
						recordedAccrual -= ev.Amount
					}
				}
			}

			// 3. Calculate expected fees on value date 'd'
			balance := l.LedgerBalanceAt(acc.ID, d)
			var expectedFee int64 = 0
			if balance < 0 && acc.Currency == "AED" {
				expectedFee = int64(OVERDRAFT_FEE * acc.Multiplier)
			}

			// If expected fee differs from recorded fee in GL, post the delta
			if expectedFee != recordedFee {
				feeDelta := expectedFee - recordedFee
				if feeDelta != 0 {
					feeEvent := &Event{
						ID:            fmt.Sprintf("FEE-DELTA-%s-D%s", acc.ID, d.Format("20060102")),
						ProcessingDay: currentOpenDay,
						ValueDate:     d,
						Type:          "FEE",
						AccountID:     acc.ID,
						Amount:        feeDelta,
						Currency:      acc.Currency,
					}
					l.ProcessEvent(feeEvent)
					fmt.Printf("[RECALC] Inserted Fee Delta: %s for Value Date %s (Expected: %s, Recorded in GL: %s)\n",
						l.formatAmount(acc.ID, feeDelta), d.Format("2006-01-02"), l.formatAmount(acc.ID, expectedFee), l.formatAmount(acc.ID, recordedFee))
				}
			}

			// 4. Calculate expected accruals on value date 'd'
			eodBalance := l.LedgerBalanceAt(acc.ID, d)
			var expectedAccrual int64 = 0
			if eodBalance > 0 {
				expectedAccrual = int64(math.Round(float64(eodBalance) * INTEREST_RATE))
			}

			// If expected accrual differs from recorded accrual in GL, post the delta
			if expectedAccrual != recordedAccrual {
				accrualDelta := expectedAccrual - recordedAccrual
				if accrualDelta != 0 {
					accrualEvent := &Event{
						ID:            fmt.Sprintf("ACCRUAL-DELTA-%s-%s", acc.ID, d.Format("20060102")),
						ProcessingDay: currentOpenDay,
						ValueDate:     d,
						Type:          "ACCRUAL",
						AccountID:     acc.ID,
						Amount:        accrualDelta,
						Currency:      acc.Currency,
					}
					l.ProcessEvent(accrualEvent)
					fmt.Printf("[RECALC] Inserted Accrual Delta: %s for Value Date %s (Balance: %s, Expected: %s, Recorded in GL: %s)\n",
						l.formatAmount(acc.ID, accrualDelta), d.Format("2006-01-02"), l.formatAmount(payableGL, eodBalance), l.formatAmount(payableGL, expectedAccrual), l.formatAmount(payableGL, recordedAccrual))
				}
			}
		}
		d = d.AddDate(0, 0, 1)
	}
}

// GetEntriesForDate returns all events affecting a specific account on a given date
func (l *Ledger) GetEntriesForDate(accId string, targetDate time.Time) []*Event {
	var matchedEvents []*Event
	for _, e := range l.Events {
		if e.AccountID == accId {
			// Check if either ProcessingDay or ValueDate matches the target date
			if e.ValueDate.Equal(targetDate) {
				matchedEvents = append(matchedEvents, e)
			}
		}
	}
	return matchedEvents
}
