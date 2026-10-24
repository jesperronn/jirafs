package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SyncedHashField is the frontmatter key holding the content hash recorded at
// the last successful export, refresh or sync of an issue file.
const SyncedHashField = "synced_hash"

// volatileFrontmatterFields are frontmatter keys excluded from the content
// hash because they change on every write without the content changing.
var volatileFrontmatterFields = []string{SyncedHashField, "sync_time"}

// splitDocument separates a rendered issue document into its frontmatter lines
// (between the two "---" delimiters) and the remaining body. ok is false when
// the document has no well-formed frontmatter.
func splitDocument(doc string) (front []string, body string, ok bool) {
	if !strings.HasPrefix(doc, "---\n") {
		return nil, "", false
	}
	rest := doc[len("---\n"):]
	lines := strings.SplitAfter(rest, "\n")
	for idx, l := range lines {
		if strings.TrimRight(l, "\r\n") == "---" {
			return lines[:idx], strings.Join(lines[idx+1:], ""), true
		}
	}
	return nil, "", false
}

func isVolatileLine(line string) bool {
	for _, f := range volatileFrontmatterFields {
		if strings.HasPrefix(line, f+":") {
			return true
		}
	}
	return false
}

// ContentHash returns the hex SHA-256 of an issue document with the volatile
// frontmatter fields (synced_hash, sync_time) removed. Documents without
// frontmatter are hashed verbatim.
func ContentHash(doc string) string {
	front, body, ok := splitDocument(doc)
	var sb strings.Builder
	if !ok {
		sb.WriteString(doc)
	} else {
		sb.WriteString("---\n")
		for _, l := range front {
			if !isVolatileLine(l) {
				sb.WriteString(l)
			}
		}
		sb.WriteString("---\n")
		sb.WriteString(body)
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// SyncedHash returns the synced_hash recorded in doc's frontmatter, or "" when
// none is recorded.
func SyncedHash(doc string) string {
	front, _, ok := splitDocument(doc)
	if !ok {
		return ""
	}
	prefix := SyncedHashField + ":"
	for _, l := range front {
		if strings.HasPrefix(l, prefix) {
			v := strings.TrimSpace(strings.TrimPrefix(l, prefix))
			v = strings.Trim(v, "'\"")
			return v
		}
	}
	return ""
}

// StampSyncedHash returns doc with a synced_hash frontmatter line holding
// ContentHash(doc), replacing any existing one. Documents without frontmatter
// are returned unchanged.
func StampSyncedHash(doc string) string {
	front, body, ok := splitDocument(doc)
	if !ok {
		return doc
	}
	h := ContentHash(doc)
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, l := range front {
		if !strings.HasPrefix(l, SyncedHashField+":") {
			sb.WriteString(l)
		}
	}
	sb.WriteString(SyncedHashField + ": " + quoteScalar(h) + "\n")
	sb.WriteString("---\n")
	sb.WriteString(body)
	return sb.String()
}

// StripVolatile returns doc with the volatile frontmatter fields (synced_hash,
// sync_time) removed. Documents without frontmatter are returned unchanged.
func StripVolatile(doc string) string {
	front, body, ok := splitDocument(doc)
	if !ok {
		return doc
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, l := range front {
		if !isVolatileLine(l) {
			sb.WriteString(l)
		}
	}
	sb.WriteString("---\n")
	sb.WriteString(body)
	return sb.String()
}
