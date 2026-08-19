// Package env is the entire interop surface exposed to interpreted guard/action
// scripts (see the parent scripting package). It is a small, hand-written,
// intentionally curated set of pure functions — never a raw stdlib package like
// "strings" wholesale, and never anything capable of I/O (no "os", "net", "io",
// "syscall"). What is NOT exported here is simply never callable from a script,
// which is the actual sandboxing mechanism: an allow-list by construction.
//
// Review any addition to this file like a trust boundary change — it directly
// controls what a database-stored script can do.
package env

import "strings"

func Contains(s, substr string) bool  { return strings.Contains(s, substr) }
func HasPrefix(s, prefix string) bool { return strings.HasPrefix(s, prefix) }
func HasSuffix(s, suffix string) bool { return strings.HasSuffix(s, suffix) }
func ToUpper(s string) string         { return strings.ToUpper(s) }
func ToLower(s string) string         { return strings.ToLower(s) }
func TrimSpace(s string) string       { return strings.TrimSpace(s) }
