package srs

import (
	"math"
	"time"

	"github.com/escoubas/zk/internal/model"
)

// Rating represents the user's rating of recall quality (0-5).
type Rating int

const (
	RatingBlackout Rating = 0 // Complete blackout
	RatingIncorrect       = 1 // Incorrect response; the correct one remembered
	RatingHard            = 2 // Correct response recalled with difficulty
	RatingPass            = 3 // Correct response recalled with moderate difficulty
	RatingGood            = 4 // Correct response after a hesitation
	RatingEasy            = 5 // Perfect recall
)

// Review updates the SRS item based on the rating using the SM-2 algorithm.
func Review(item *model.SRSItem, rating Rating) {
	if item.EaseFactor < 1.3 {
		item.EaseFactor = 2.5 // Default starting EF
	}

	if rating >= RatingPass {
		if item.Repetitions == 0 {
			item.Interval = 1
		} else if item.Repetitions == 1 {
			item.Interval = 6
		} else {
			item.Interval = math.Round(item.Interval * item.EaseFactor)
		}
		item.Repetitions++
	} else {
		item.Repetitions = 0
		item.Interval = 1
	}

	// Update Ease Factor
	// EF' = EF + (0.1 - (5 - q) * (0.08 + (5 - q) * 0.02))
	q := float64(rating)
	item.EaseFactor = item.EaseFactor + (0.1 - (5-q)*(0.08+(5-q)*0.02))
	if item.EaseFactor < 1.3 {
		item.EaseFactor = 1.3
	}

	// Update NextReview
	// Add interval days to Now (or to previous scheduled date?)
	// SM-2 typically schedules from "Review Date".
	item.NextReview = time.Now().Add(time.Duration(item.Interval) * 24 * time.Hour)
}

// InitialState returns a new SRS item state.
func InitialState(noteID string) *model.SRSItem {
	return &model.SRSItem{
		NoteID:      noteID,
		NextReview:  time.Now(), // Due immediately
		Interval:    0,
		EaseFactor:  2.5,
		Repetitions: 0,
	}
}
