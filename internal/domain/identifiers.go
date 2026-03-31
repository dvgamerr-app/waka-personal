package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func BuildDedupeIdentifiers(
	sourceHeartbeatID string,
	heartbeatTime time.Time,
	entity, entityType, category, project, branch, language string,
	isWrite bool,
	lineno, cursorpos *int,
	plugin string,
) (string, string) {
	if sourceHeartbeatID != "" {
		hash := sha256.Sum256([]byte("src:" + sourceHeartbeatID))
		return uuid.NewString(), hex.EncodeToString(hash[:])
	}

	parts := []string{
		heartbeatTime.UTC().Format(time.RFC3339Nano),
		entity,
		entityType,
		category,
		project,
		branch,
		language,
		fmt.Sprintf("%t", isWrite),
		fmt.Sprintf("%d", derefInt(lineno)),
		fmt.Sprintf("%d", derefInt(cursorpos)),
		plugin,
	}
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return uuid.NewString(), hex.EncodeToString(hash[:])
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
