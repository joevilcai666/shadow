package storage

import (
	"math"
	"time"
)

// DecayHalfLife is the half-life for rule decay (30 days).
const DecayHalfLife = 30 * 24 * time.Hour

// ComputeDecayScore calculates the decay score for a rule.
// Formula: decay_score = confidence * (0.5 ^ (days_since_last_hit / half_life_days))
// If lastHitAt is empty (never hit), returns the initial confidence.
func ComputeDecayScore(initialConfidence float64, lastHitAt string) float64 {
	if lastHitAt == "" {
		return initialConfidence
	}

	t, err := time.Parse(time.RFC3339, lastHitAt)
	if err != nil {
		return initialConfidence
	}

	daysSinceHit := time.Since(t).Hours() / 24.0
	if daysSinceHit < 0 {
		// Future date: use full confidence
		return initialConfidence
	}

	decayFactor := math.Pow(0.5, daysSinceHit/30.0)
	return initialConfidence * decayFactor
}

// RuleDecayInfo holds decay-related fields from a rule row.
type RuleDecayInfo struct {
	ID            string
	Confidence    float64
	DecayScore    float64
	LastHitAt     string
	Status        string
}

// RecomputeAllDecayScores updates decay_score for all active rules.
// Returns the count of rules updated.
func RecomputeAllDecayScores(db DB) (int, error) {
	rows, err := db.Query(`
		SELECT id, confidence, COALESCE(last_hit_at,'') FROM rules
		WHERE status = 'active' AND last_hit_at IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toUpdate []RuleDecayInfo
	for rows.Next() {
		var r RuleDecayInfo
		if err := rows.Scan(&r.ID, &r.Confidence, &r.LastHitAt); err != nil {
			continue
		}
		r.DecayScore = ComputeDecayScore(r.Confidence, r.LastHitAt)
		toUpdate = append(toUpdate, r)
	}

	for _, r := range toUpdate {
		_, err := db.Exec(`UPDATE rules SET decay_score = ? WHERE id = ?`, r.DecayScore, r.ID)
		if err != nil {
			return 0, err
		}
	}
	return len(toUpdate), nil
}