package daemon

import (
	"database/sql"
	"time"

	"github.com/joevilcai666/shadow/internal/storage"
)

// HealthStats holds memory layer health statistics.
type HealthStats struct {
	TotalRules      int            `json:"total_rules"`
	ActiveRules     int            `json:"active_rules"`
	CandidateRules  int            `json:"candidate_rules"`
	DisabledRules   int            `json:"disabled_rules"`
	ConflictedRules int            `json:"conflicted_rules"`
	LowHitRules     int            `json:"low_hit_rules"` // rules with decay_score < 0.3
	GlobalRules     int            `json:"global_rules"`
	ProjectRules    int            `json:"project_rules"`
	HitRate         float64        `json:"hit_rate"`         // 0-100%
	HitRateTrend    string         `json:"hit_rate_trend"`   // "up" | "down" | "stable"
	LastHit         *LastHitInfo    `json:"last_hit"`
	AdapterSyncs    []AdapterSync  `json:"adapter_syncs"`
	GeneratedAt     string         `json:"generated_at"`
}

// LastHitInfo describes the most recent rule hit.
type LastHitInfo struct {
	RuleID      string `json:"rule_id"`
	RuleContent string `json:"rule_content"`
	AgentName   string `json:"agent_name"`
	ProjectPath string `json:"project_path"`
	Timestamp   string `json:"timestamp"`
}

// AdapterSync describes the last sync status for an adapter.
type AdapterSync struct {
	AgentName   string `json:"agent_name"`
	LastSync    string `json:"last_sync"`
	Status      string `json:"status"` // "ok" | "stale" | "error"
	LastSuccess string `json:"last_success,omitempty"`
}

// GetHealthStats retrieves memory layer health statistics.
func GetHealthStats(db *sql.DB) (*HealthStats, error) {
	ruleRepo := storage.NewRuleRepo(db)
	eventRepo := storage.NewEventRepo(db)

	// Count rules by status.
	active, _ := ruleRepo.Count(storage.RuleFilter{Status: "active"})
	candidate, _ := ruleRepo.Count(storage.RuleFilter{Status: "candidate"})
	disabled, _ := ruleRepo.Count(storage.RuleFilter{Status: "disabled"})
	conflicted, _ := ruleRepo.Count(storage.RuleFilter{Status: "conflicted"})
	global, _ := ruleRepo.Count(storage.RuleFilter{Scope: "global"})
	project, _ := ruleRepo.Count(storage.RuleFilter{Scope: "project"})
	total := active + candidate + disabled + conflicted

	// Count low-hit rules (decay_score < 0.3).
	lowHit := 0
	if db != nil {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM rules WHERE status='active' AND COALESCE(decay_score,0) < 0.3`).Scan(&count)
		lowHit = count
	}

	// Calculate hit rate (rule_hit events in last 7 days / total active rules).
	hitsThisWeek := 0
	hitsLastWeek := 0
	weekAgo := time.Now().AddDate(0, 0, -7).Format(time.RFC3339)
	twoWeeksAgo := time.Now().AddDate(0, 0, -14).Format(time.RFC3339)

	if eventRepo != nil && db != nil {
		db.QueryRow(`SELECT COUNT(*) FROM events WHERE event_type='rule_hit' AND timestamp >= ?`, weekAgo).Scan(&hitsThisWeek)
		db.QueryRow(`SELECT COUNT(*) FROM events WHERE event_type='rule_hit' AND timestamp >= ? AND timestamp < ?`, twoWeeksAgo, weekAgo).Scan(&hitsLastWeek)
	}

	var hitRate float64
	if total > 0 {
		hitRate = float64(hitsThisWeek) / float64(total) * 100
		if hitRate > 100 {
			hitRate = 100
		}
	}

	var trend string
	if hitsThisWeek > hitsLastWeek {
		trend = "up"
	} else if hitsThisWeek < hitsLastWeek {
		trend = "down"
	} else {
		trend = "stable"
	}

	// Get last hit info.
	var lastHit *LastHitInfo
	if db != nil {
		var ruleID, agentName, projectPath, timestamp, content string
		err := db.QueryRow(`
			SELECT e.rule_id, COALESCE(e.agent_name,''), COALESCE(e.project_path,''),
			       e.timestamp, COALESCE(r.content,'')
			FROM events e
			LEFT JOIN rules r ON e.rule_id = r.id
			WHERE e.event_type = 'rule_hit'
			ORDER BY e.timestamp DESC LIMIT 1`).Scan(&ruleID, &agentName, &projectPath, &timestamp, &content)
		if err == nil && ruleID != "" {
			lastHit = &LastHitInfo{
				RuleID:      ruleID,
				RuleContent: truncate(content, 60),
				AgentName:   agentName,
				ProjectPath: projectPath,
				Timestamp:   timestamp,
			}
		}
	}

	// Get adapter sync status.
	adapterSyncs := []AdapterSync{
		{AgentName: "claude-code", LastSync: "unknown", Status: "stale"},
		{AgentName: "cursor", LastSync: "unknown", Status: "stale"},
		{AgentName: "codex", LastSync: "unknown", Status: "stale"},
		{AgentName: "copilot", LastSync: "unknown", Status: "stale"},
	}
	if db != nil {
		rows, _ := db.Query(`
			SELECT agent_name, MAX(timestamp), event_type
			FROM events
			WHERE event_type IN ('sync_success', 'sync_failure')
			GROUP BY agent_name`)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var agent, ts, et string
				rows.Scan(&agent, &ts, &et)
				for i := range adapterSyncs {
					if adapterSyncs[i].AgentName == agent {
						adapterSyncs[i].LastSync = ts
						if et == "sync_success" {
							adapterSyncs[i].Status = "ok"
							adapterSyncs[i].LastSuccess = ts
						} else {
							adapterSyncs[i].Status = "error"
						}
					}
				}
			}
		}
	}

	return &HealthStats{
		TotalRules:      total,
		ActiveRules:     active,
		CandidateRules:  candidate,
		DisabledRules:   disabled,
		ConflictedRules:  conflicted,
		LowHitRules:     lowHit,
		GlobalRules:     global,
		ProjectRules:    project,
		HitRate:         hitRate,
		HitRateTrend:    trend,
		LastHit:         lastHit,
		AdapterSyncs:    adapterSyncs,
		GeneratedAt:     time.Now().Format(time.RFC3339),
	}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}