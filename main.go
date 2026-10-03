package main

import (
	"fmt"
	"math"
	"time"
)

type EventType string

const (
	Credit        EventType = "CREDIT"
	Debit         EventType = "DEBIT"
	Authorization EventType = "AUTHORIZATION"
	Settlement    EventType = "SETTLEMENT"
	Reversal      EventType = "REVERSAL"
	Fee           EventType = "FEE"
	Interest      EventType = "INTEREST"
	Accrual       EventType = "ACCRUAL"
)

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
	ID         string
	Currency   string
	Precision  int
	Multiplier int64
	IsInternal bool // True for GL/accrual accounts
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

func (l *Ledger) AddAccount(id, currency string, precision int, isInternal bool) {
	mult := int64(math.Pow10(precision))
	l.Accounts[id] = &Account{
		ID:         id,
		Currency:   currency,
		Precision:  precision,
		Multiplier: mult,
		IsInternal: isInternal,
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
	var balance int64 = 0
	for _, e := range l.Events {
		if e.AccountID == accId && onOrBefore(e.ValueDate, asOf) {
			if e.Type == Credit || e.Type == Settlement || e.Type == Interest || e.Type == Accrual {
				balance += e.Amount
			} else if e.Type == Debit || e.Type == Fee {
				balance -= e.Amount
			} else if e.Type == Reversal {
				targetevent := GetEvent(e.TargetEventID, l.Events)
				if targetevent != nil {
					if targetevent.Type == Debit || targetevent.Type == Fee {
						balance += targetevent.Amount
					} else {
						balance -= targetevent.Amount
					}
				}
			}
		}
	}
	return balance
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

func (l *Ledger) ProcessEvent(e *Event) error {
	acc := l.Accounts[e.AccountID]

	switch e.Type {
	case Authorization:
		avail := l.AvailableBalanceAt(acc.ID, e.ProcessingDay)
		if avail-e.Amount < 0 {
			return fmt.Errorf("authorization declined: insufficient available balance")
		}
		l.ActiveHolds[acc.ID][e.RefID] = e.Amount
		l.Events = append(l.Events, e)

	case Settlement:
		delete(l.ActiveHolds[acc.ID], e.RefID)
		l.Events = append(l.Events, e)

	case Credit, Debit, Reversal, Accrual, Interest, Fee:
		l.Events = append(l.Events, e)
	}

	return nil
}

// PrintLedgerState prints the entire snapshot of the ledger including internal GL accounts
func (l *Ledger) PrintLedgerState(asOf time.Time) {
	fmt.Println("    | --- LEDGER SNAPSHOT ---")

	accIDs := []string{"ACC-001", "ACC-002", "GL-INT-PAYABLE-ACC-001"}
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

func (l *Ledger) EndOfDay(currentDay time.Time, isLastDay bool) {
	fmt.Printf("\n>>> End of Day %s Processing <<<\n", currentDay.Format("2006-01-02"))

	for _, acc := range l.Accounts {
		if acc.IsInternal {
			continue // Skip internal accounts for customer EOD rules
		}

		// 1. Overdraft fee evaluations
		balance := l.LedgerBalanceAt(acc.ID, currentDay)
		if balance < 0 && !l.FeesAssessed[acc.ID][currentDay] {
			// for d := l.StartDate; onOrBefore(d, currentDay); d = d.AddDate(0, 0, 1) {
			// balance := l.LedgerBalanceAt(acc.ID, d)
			// if balance < 0 && !l.FeesAssessed[acc.ID][d] {
			feeAmount := int64(25 * acc.Multiplier)
			if acc.Currency == "AED" {
				feeEvent := &Event{
					ID:            fmt.Sprintf("FEE-%s-D%s", acc.ID, currentDay.Format("20060102")),
					ProcessingDay: currentDay,
					ValueDate:     currentDay,
					Type:          Fee,
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
			// }
		}

		// 2. Daily Interest Accrual into Internal GL Account
		eodBalance := l.LedgerBalanceAt(acc.ID, currentDay)
		if eodBalance > 0 {
			exactAccrual := float64(eodBalance) * 0.0004
			roundedAccrual := int64(math.Round(exactAccrual))
			if roundedAccrual > 0 {
				glAccID := "GL-INT-PAYABLE-" + acc.ID
				accrualEvent := &Event{
					ID:            fmt.Sprintf("ACCRUAL-%s-%s", acc.ID, currentDay.Format("20060102")),
					ProcessingDay: currentDay,
					ValueDate:     currentDay,
					Type:          Accrual,
					AccountID:     glAccID,
					Amount:        roundedAccrual,
					Currency:      acc.Currency,
				}
				l.ProcessEvent(accrualEvent)
				fmt.Printf("Accrued Interest: %s stored in %s\n", l.formatAmount(acc.ID, roundedAccrual), glAccID)
				l.PrintLedgerState(currentDay)
			}
		}

		// 3. Capitalize Interest on Final Day
		if isLastDay {
			glAccID := "GL-INT-PAYABLE-" + acc.ID
			totalAccrued := l.LedgerBalanceAt(glAccID, currentDay)
			if totalAccrued > 0 {
				// Capitalization moves funds from GL Payable to Customer Account
				interestEvent := &Event{
					ID:            fmt.Sprintf("INT-%s", acc.ID),
					ProcessingDay: currentDay,
					ValueDate:     currentDay,
					Type:          Interest,
					AccountID:     acc.ID,
					Amount:        totalAccrued,
					Currency:      acc.Currency,
				}
				l.ProcessEvent(interestEvent)
				fmt.Printf("Capitalized Interest: %s transferred from %s to %s\n", l.formatAmount(acc.ID, totalAccrued), glAccID, acc.ID)
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
	l.AddAccount("ACC-001", "AED", 2, false)
	l.AddAccount("ACC-002", "BHD", 3, false)
	l.AddAccount("GL-INT-PAYABLE-ACC-001", "AED", 2, true) // Internal GL account for tracking accrued liability

	stream := []*Event{
		{ID: "E1", ProcessingDay: days[0], ValueDate: days[0], Type: Credit, AccountID: "ACC-001", Amount: 120000, Currency: "AED"},
		{ID: "E2", ProcessingDay: days[0], ValueDate: days[0], Type: Debit, AccountID: "ACC-001", Amount: 95000, Currency: "AED"},
		{ID: "E3", ProcessingDay: days[1], ValueDate: days[1], Type: Authorization, AccountID: "ACC-001", Amount: 20000, Currency: "AED", RefID: "Auth-A"},
		{ID: "E4", ProcessingDay: days[2], ValueDate: days[2], Type: Credit, AccountID: "ACC-001", Amount: 40000, Currency: "AED"},
		{ID: "E5", ProcessingDay: days[3], ValueDate: days[3], Type: Settlement, AccountID: "ACC-001", Amount: 18500, Currency: "AED", RefID: "Auth-A"},
		{ID: "E6", ProcessingDay: days[3], ValueDate: days[3], Type: Settlement, AccountID: "ACC-001", Amount: 18000, Currency: "AED", RefID: "Auth-Z"},
		{ID: "E7", ProcessingDay: days[4], ValueDate: days[1], Type: Debit, AccountID: "ACC-001", Amount: 62000, Currency: "AED"},
		{ID: "E8", ProcessingDay: days[4], ValueDate: days[4], Type: Authorization, AccountID: "ACC-001", Amount: 9000, Currency: "AED", RefID: "Auth-B"},
		{ID: "E9", ProcessingDay: days[5], ValueDate: days[1], Type: Reversal, AccountID: "ACC-001", TargetEventID: "E7", Currency: "AED"},
		{ID: "E10-1", ProcessingDay: days[4], ValueDate: days[4], Type: Credit, AccountID: "ACC-002", Amount: 3334, Currency: "BHD"},
		{ID: "E10-2", ProcessingDay: days[4], ValueDate: days[4], Type: Credit, AccountID: "ACC-002", Amount: 3333, Currency: "BHD"},
		{ID: "E10-3", ProcessingDay: days[4], ValueDate: days[4], Type: Credit, AccountID: "ACC-002", Amount: 3333, Currency: "BHD"},
	}

	currentOpenDay := days[0]

	fmt.Printf("\n==========================================\n")
	fmt.Printf(" STARTING DAY: %s \n", currentOpenDay.Format("2006-01-02"))
	fmt.Printf("==========================================\n")

	for _, e := range stream {
		// 1. Check for late-arriving events (e.g., Day 5 event arriving while Day 6 is open)
		if e.ProcessingDay.Before(currentOpenDay) {
			fmt.Printf("\n[!] LATE EVENT DETECTED: %s (Adjusting Processing Day %s -> %s)\n",
				e.ID, e.ProcessingDay.Format("2006-01-02"), currentOpenDay.Format("2006-01-02"))
			e.ProcessingDay = currentOpenDay
		}

		// 2. Check for day advancement (close current day if the next event is for a future day)
		for e.ProcessingDay.After(currentOpenDay) {
			isLastDay := currentOpenDay.Equal(days[len(days)-1])
			l.EndOfDay(currentOpenDay, isLastDay)

			currentOpenDay = currentOpenDay.AddDate(0, 0, 1)

			// Only print "STARTING DAY" if we are still within our 6-day window
			if onOrBefore(currentOpenDay, days[len(days)-1]) {
				fmt.Printf("\n==========================================\n")
				fmt.Printf(" STARTING DAY: %s \n", currentOpenDay.Format("2006-01-02"))
				fmt.Printf("==========================================\n")
			}
		}

		// 3. Process the event
		err := l.ProcessEvent(e)
		if err != nil {
			fmt.Printf("\n[X] EVENT FAILED: %s (%v)\n", e.ID, err)
		} else {
			if e.Type == Reversal {
				targetevent := GetEvent(e.TargetEventID, l.Events)
				if targetevent != nil {
					fmt.Printf("\n[+] PROCESSED EVENT: %s (%s %s -> %s %s) Value Date: %s\n", e.ID, e.Type, e.TargetEventID, targetevent.Type, l.formatAmount(targetevent.AccountID, targetevent.Amount), e.ValueDate.Format("2006-01-02"))
				}
			} else {
				fmt.Printf("\n[+] PROCESSED EVENT: %s (%s %s %s %s) Value Date: %s\n", e.ID, e.Type, l.formatAmount(e.AccountID, e.Amount), e.RefID, e.TargetEventID, e.ValueDate.Format("2006-01-02"))
			}
			// Dump the full state immediately after processing the event
			l.PrintLedgerState(currentOpenDay)
		}
	}

	// 4. Close any remaining days in the window (just in case the stream ended early)
	for onOrBefore(currentOpenDay, days[len(days)-1]) {
		isLastDay := currentOpenDay.Equal(days[len(days)-1])
		l.EndOfDay(currentOpenDay, isLastDay)

		if !isLastDay {
			currentOpenDay = currentOpenDay.AddDate(0, 0, 1)
			fmt.Printf("\n==========================================\n")
			fmt.Printf(" STARTING DAY: %s \n", currentOpenDay.Format("2006-01-02"))
			fmt.Printf("==========================================\n")
		} else {
			break
		}
	}
}
