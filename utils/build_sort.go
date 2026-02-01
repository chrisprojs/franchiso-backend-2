package utils

func BuildSort(orderBy *string, orderDirection *string) []map[string]interface{} {
	// 1. Handle Explicit Sorting (User selected a field)
	if orderBy != nil {
		allowedFields := map[string]string{
			"investment":      "investment",
			"monthly_revenue": "monthly_revenue",
			"roi":             "roi",
			"branch_count":    "branch_count",
			"year_founded":    "year_founded",
			"created_at":      "created_at",
		}

		if field, ok := allowedFields[*orderBy]; ok {
			direction := "asc"
			if orderDirection != nil && (*orderDirection == "desc" || *orderDirection == "asc") {
				direction = *orderDirection
			}

			// Return ONLY the requested sort (or add _score as a fallback tie-breaker)
			return []map[string]interface{}{
				{field: map[string]interface{}{"order": direction}},
				{"_score": map[string]interface{}{"order": "desc"}},
			}
		}
	}

	// 2. Default Sort (When no sortBy is provided)
	// Prioritize boosted items and relevance score
	return []map[string]interface{}{
		{"is_boosted": map[string]interface{}{"order": "desc"}},
		{"_score": map[string]interface{}{"order": "desc"}},
	}
}
