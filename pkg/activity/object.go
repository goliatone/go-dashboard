package activity

import "strings"

// ParseCompositeObject parses a composite identifier in the form "type:id".
//
// Behavior:
// - No colon: returns (trimmed input, "", false).
// - Multiple colons: splits on the first colon; the remainder is objectID.
// - Empty parts: returns trimmed parts with ok=false when either side is empty.
func ParseCompositeObject(input string) (objectType, objectID string, ok bool) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", false
	}

	typ, id, found := strings.Cut(value, ":")
	if !found {
		return value, "", false
	}

	objectType = strings.TrimSpace(typ)
	objectID = strings.TrimSpace(id)
	return objectType, objectID, objectType != "" && objectID != ""
}
