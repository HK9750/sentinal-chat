package services

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

func chatDedupeUUIDs(items []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(items))
	result := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		if item == uuid.Nil {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func chatNormalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func chatNormalizeLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > fallback {
		return fallback
	}
	return limit
}

func chatNullableString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func chatNullableUUID(value uuid.UUID) uuid.NullUUID {
	if value == uuid.Nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: value, Valid: true}
}

func chatNullableTimePtr(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func minUUID(a, b uuid.UUID) uuid.UUID {
	if a.String() < b.String() {
		return a
	}
	return b
}

func maxUUID(a, b uuid.UUID) uuid.UUID {
	if a.String() > b.String() {
		return a
	}
	return b
}
