package srs

import (
	"math"
	"testing"
)

func TestInitialState(t *testing.T) {
	item := InitialState("note-1")
	if item.NoteID != "note-1" {
		t.Errorf("expected ID note-1, got %s", item.NoteID)
	}
	if item.Interval != 0 {
		t.Errorf("expected interval 0, got %f", item.Interval)
	}
	if item.EaseFactor != 2.5 {
		t.Errorf("expected EF 2.5, got %f", item.EaseFactor)
	}
	if item.Repetitions != 0 {
		t.Errorf("expected Repetitions 0, got %d", item.Repetitions)
	}
}

func TestReview(t *testing.T) {
	item := InitialState("note-1")

	// 1st Review: Pass (3)
	Review(item, RatingPass)
	if item.Repetitions != 1 {
		t.Errorf("expected reps 1, got %d", item.Repetitions)
	}
	if item.Interval != 1 {
		t.Errorf("expected interval 1, got %f", item.Interval)
	}
	// EF should decrease slightly? 
	// q=3: EF' = 2.5 + (0.1 - (2) * (0.08 + 2*0.02)) = 2.5 + (0.1 - 2 * 0.12) = 2.5 + (0.1 - 0.24) = 2.5 - 0.14 = 2.36
	if math.Abs(item.EaseFactor - 2.36) > 0.01 {
		t.Errorf("expected EF approx 2.36, got %f", item.EaseFactor)
	}

	// 2nd Review: Good (4)
	Review(item, RatingGood)
	if item.Repetitions != 2 {
		t.Errorf("expected reps 2, got %d", item.Repetitions)
	}
	if item.Interval != 6 {
		t.Errorf("expected interval 6, got %f", item.Interval)
	}
	
	// 3rd Review: Easy (5)
	// Previous EF was approx 2.36
	// q=4 for prev (Good) actually updated EF?
	// Wait, Review(item, RatingGood) updates EF too.
	// q=4: EF' = EF + (0.1 - (1)*(0.08 + 1*0.02)) = EF + 0 = EF.
	// So EF is still 2.36.
	
	Review(item, RatingEasy)
	if item.Repetitions != 3 {
		t.Errorf("expected reps 3, got %d", item.Repetitions)
	}
	// Interval = previous_interval * EF = 6 * 2.36 = 14.16 -> rounded 14
	if item.Interval != 14 {
		t.Errorf("expected interval 14, got %f", item.Interval)
	}
	
	// Fail (1)
	Review(item, RatingIncorrect)
	if item.Repetitions != 0 {
		t.Errorf("expected reps 0 after fail, got %d", item.Repetitions)
	}
	if item.Interval != 1 {
		t.Errorf("expected interval 1 after fail, got %f", item.Interval)
	}
}
