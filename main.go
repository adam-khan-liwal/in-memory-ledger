package main

import (
	"fmt"
	"math"
	"sort"
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
)

type Event struct {
	ID            string
	ProcessingDay time.Time
	ValueDate     time.Time
	Type          EventType
	AccountID     string
	Amount        int64 // Stored in minor units (cents/fils)
	Currency      string
	RefID         string // For settlements/reversals
	TargetEventID string // For reversals
}

type Account struct {
	ID         string
	Currency   string
	Precision  int
	Multiplier int64
}

type Ledger struct {
	StartDate       time.Time
	Accounts        map[string]*Account
	Events          []*Event
	ActiveHolds     map[string]map[string]int64   // AccountID -> AuthID -> Amount
	FeesAssessed    map[string]map[time.Time]bool // AccountID -> ValueDate -> Assessed?
	AccruedInterest map[string]int64              // AccountID -> Total Accrued
	DailyAccruals   map[string][]int64            // AccountID -> List of daily accruals
}

func NewLedger(startDate time.Time) *Ledger {
	return &Ledger{
		StartDate:       startDate,
		Accounts:        make(map[string]*Account),
		Events:          make([]*Event, 0),
		ActiveHolds:     make(map[string]map[string]int64),
		FeesAssessed:    make(map[string]map[time.Time]bool),
		AccruedInterest: make(map[string]int64),
		DailyAccruals:   make(map[string][]int64),
	}
}

func (l *Ledger) AddAccount(id, currency string, precision int) {
	mult := int64(math.Pow10(precision))
	l.Accounts[id] = &Account{ID: id, Currency: currency, Precision: precision, Multiplier: mult}
	l.ActiveHolds[id] = make(map[string]int64)
	l.FeesAssessed[id] = make(map[time.Time]bool)
}

func (l *Ledger) formatAmount(accId string, amount int64) string {
	acc := l.Accounts[accId]
	format := fmt.Sprintf("%%.%df", acc.Precision)
	return fmt.Sprintf("%s "+format, acc.Currency, float64(amount)/float64(acc.Multiplier))
}

// Helper to check if t1 is on or before t2
func onOrBefore(t1, t2 time.Time) bool {
	return t1.Before(t2) || t1.Equal(t2)
}

func (l *Ledger) LedgerBalanceAt(accId string, asOf time.Time) int64 {
	var balance int64 = 0
	for _, e := range l.Events {
		if e.AccountID == accId && onOrBefore(e.ValueDate, asOf) {
			if e.Type == Credit || e.Type == Settlement || e.Type == Interest {
				balance += e.Amount
			} else if e.Type == Debit || e.Type == Fee {
				balance -= e.Amount
			} else if e.Type == Reversal {
				// Find target
				for _, target := range l.Events {
					if target.ID == e.TargetEventID {
						if target.Type == Debit || target.Type == Fee {
							balance += target.Amount
						} else {
							balance -= target.Amount
						}
						break
					}
				}
			}
		}
	}
	return balance
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
		// Settle and clear hold if it exists (allow force post if it doesn't)
		delete(l.ActiveHolds[acc.ID], e.RefID)
		l.Events = append(l.Events, e)

	case Credit, Debit, Reversal:
		l.Events = append(l.Events, e)
	}

	return nil
}

func (l *Ledger) EndOfDay(currentDay time.Time, isLastDay bool) {
	fmt.Printf("\n--- End of Day %s ---\n", currentDay.Format("2006-01-02"))

	for _, acc := range l.Accounts {
		// 1. Evaluate historical balances for overdraft fees up to currentDay
		for d := l.StartDate; onOrBefore(d, currentDay); d = d.AddDate(0, 0, 1) {
			balance := l.LedgerBalanceAt(acc.ID, d)
			if balance < 0 && !l.FeesAssessed[acc.ID][d] {
				feeAmount := int64(25 * acc.Multiplier) // AED 25
				if acc.Currency == "AED" {
					feeEvent := &Event{
						ID:            fmt.Sprintf("FEE-%s-D%s", acc.ID, d.Format("20060102")),
						ProcessingDay: currentDay,
						ValueDate:     currentDay, // Assessed and booked on currentDay
						Type:          Fee,
						AccountID:     acc.ID,
						Amount:        feeAmount,
						Currency:      acc.Currency,
					}
					l.Events = append(l.Events, feeEvent)
					l.FeesAssessed[acc.ID][d] = true
					fmt.Printf("Assessed Overdraft Fee: %s on %s (Triggered by Value Date %s balance)\n",
						l.formatAmount(acc.ID, feeAmount), currentDay.Format("2006-01-02"), d.Format("2006-01-02"))
				}
			}
		}

		// 2. Accrue Interest for currentDay
		eodBalance := l.LedgerBalanceAt(acc.ID, currentDay)
		if eodBalance > 0 {
			exactAccrual := float64(eodBalance) * 0.0004
			roundedAccrual := int64(math.Round(exactAccrual))
			l.AccruedInterest[acc.ID] += roundedAccrual
			l.DailyAccruals[acc.ID] = append(l.DailyAccruals[acc.ID], roundedAccrual)
		} else {
			l.DailyAccruals[acc.ID] = append(l.DailyAccruals[acc.ID], 0)
		}

		// 3. Capitalize Interest on final day
		if isLastDay {
			totalAccrued := l.AccruedInterest[acc.ID]
			if totalAccrued > 0 {
				sumDaily := int64(0)
				for _, a := range l.DailyAccruals[acc.ID] {
					sumDaily += a
				}

				// Reconciliation / adjustment of last accrual to match total if needed
				diff := totalAccrued - sumDaily
				if diff != 0 {
					totalAccrued = sumDaily + diff // Ensures sum matches capitalized exactly
				}

				interestEvent := &Event{
					ID:            fmt.Sprintf("INT-%s", acc.ID),
					ProcessingDay: currentDay,
					ValueDate:     currentDay,
					Type:          Interest,
					AccountID:     acc.ID,
					Amount:        totalAccrued,
					Currency:      acc.Currency,
				}
				l.Events = append(l.Events, interestEvent)
				fmt.Printf("Capitalized Interest: %s\n", l.formatAmount(acc.ID, totalAccrued))
			}
		}

		// Output EOD State
		ledgerBal := l.LedgerBalanceAt(acc.ID, currentDay)
		availBal := l.AvailableBalanceAt(acc.ID, currentDay)
		fmt.Printf("%s - Ledger Balance: %s | Available Balance: %s\n", acc.ID, l.formatAmount(acc.ID, ledgerBal), l.formatAmount(acc.ID, availBal))
	}
}

func main() {
	// Establish our 6-day window using time.Time exactly at UTC midnight to ensure clean day boundaries
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
	l.AddAccount("ACC-001", "AED", 2)
	l.AddAccount("ACC-002", "BHD", 3)

	stream := []*Event{
		{ID: "E1", ProcessingDay: days[0], ValueDate: days[0], Type: Credit, AccountID: "ACC-001", Amount: 120000, Currency: "AED"},
		{ID: "E2", ProcessingDay: days[0], ValueDate: days[0], Type: Debit, AccountID: "ACC-001", Amount: 95000, Currency: "AED"},
		{ID: "E3", ProcessingDay: days[1], ValueDate: days[1], Type: Authorization, AccountID: "ACC-001", Amount: 20000, Currency: "AED", RefID: "Auth-A"},
		{ID: "E4", ProcessingDay: days[2], ValueDate: days[2], Type: Credit, AccountID: "ACC-001", Amount: 40000, Currency: "AED"},
		{ID: "E5", ProcessingDay: days[3], ValueDate: days[3], Type: Settlement, AccountID: "ACC-001", Amount: 18500, Currency: "AED", RefID: "Auth-A"},
		{ID: "E6", ProcessingDay: days[3], ValueDate: days[3], Type: Settlement, AccountID: "ACC-001", Amount: 18000, Currency: "AED", RefID: "Auth-Z"},
		{ID: "E7", ProcessingDay: days[4], ValueDate: days[1], Type: Debit, AccountID: "ACC-001", Amount: 62000, Currency: "AED"},
		{ID: "E8", ProcessingDay: days[4], ValueDate: days[4], Type: Authorization, AccountID: "ACC-001", Amount: 9000, Currency: "AED", RefID: "Auth-B"},
		{ID: "E10-1", ProcessingDay: days[4], ValueDate: days[4], Type: Credit, AccountID: "ACC-002", Amount: 3334, Currency: "BHD"},
		{ID: "E10-2", ProcessingDay: days[4], ValueDate: days[4], Type: Credit, AccountID: "ACC-002", Amount: 3333, Currency: "BHD"},
		{ID: "E10-3", ProcessingDay: days[4], ValueDate: days[4], Type: Credit, AccountID: "ACC-002", Amount: 3333, Currency: "BHD"},
		{ID: "E9", ProcessingDay: days[5], ValueDate: days[1], Type: Reversal, AccountID: "ACC-001", TargetEventID: "E7", Currency: "AED"},
	}

	// Process event stream chronologically grouped by processing day
	for i, day := range days {
		fmt.Printf("\n--- Processing Events for %s ---\n", day.Format("2006-01-02"))

		var dailyEvents []*Event
		for _, e := range stream {
			if e.ProcessingDay.Equal(day) {
				dailyEvents = append(dailyEvents, e)
			}
		}

		// Sort by ID logically to ensure deterministic runs
		sort.SliceStable(dailyEvents, func(a, b int) bool { return dailyEvents[a].ID < dailyEvents[b].ID })

		for _, e := range dailyEvents {
			err := l.ProcessEvent(e)
			if err != nil {
				fmt.Printf("Event %s Error: %v\n", e.ID, err)
			} else {
				fmt.Printf("Processed %s: %s %s\n", e.ID, e.Type, l.formatAmount(e.AccountID, e.Amount))
			}
		}

		// Evaluate and print EOD totals, fee assessments, and accruals
		isLastDay := i == len(days)-1
		l.EndOfDay(day, isLastDay)
	}
}
