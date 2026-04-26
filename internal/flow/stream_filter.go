package flow

import "strings"

// streamTokenFilter wraps multiple dropTagParsers to filter
// one or more named tags from a streaming token sequence.
type streamTokenFilter struct {
	parsers []*dropTagParser
}

// newStreamTokenFilter creates a filter for all provided tag names.
// If tags is nil/empty, the filter is a no-op pass-through.
func newStreamTokenFilter(tags []string) *streamTokenFilter {
	f := &streamTokenFilter{}
	for _, tag := range tags {
		if tag != "" {
			f.parsers = append(f.parsers, newDropTagParser(tag))
		}
	}
	return f
}

// Apply runs the event content through all tag filters and returns
// a new StreamEvent with the filtered content.
func (f *streamTokenFilter) Apply(ev StreamEvent) StreamEvent {
	if len(f.parsers) == 0 || ev.Content == "" {
		return ev
	}
	text := ev.Content
	for _, p := range f.parsers {
		text = p.Consume(text)
	}
	ev.Content = text
	return ev
}

// Flush drains any buffered partial tag data from all parsers.
// It returns any un-emitted plain text that was held back waiting
// for a possible tag boundary.
func (f *streamTokenFilter) Flush(suffix string) string {
	if len(f.parsers) == 0 {
		return suffix
	}
	// Pass suffix through all parsers with Consume, then Flush each.
	text := suffix
	for _, p := range f.parsers {
		if text != "" {
			text = p.Consume(text)
		}
		text += p.Flush()
	}
	return text
}

// SplitByTagPrefix splits s into a safe-to-emit prefix and a carry
// portion that must be buffered because it could be the start of tag.
//
// Example:
//
//	s     = "hello<xaiarti"
//	tag   = "<xaiartifact"
//	→ prefix="hello", carry="<xaiarti"
//
// If no partial tag prefix is found at the tail of s, carry is "".
func SplitByTagPrefix(s, tag string) (prefix, carry string) {
	if s == "" || tag == "" {
		return s, ""
	}
	// Walk backwards to find the longest suffix of s that is a prefix of tag.
	maxCheck := len(tag) - 1
	if maxCheck > len(s) {
		maxCheck = len(s)
	}
	for length := maxCheck; length >= 1; length-- {
		tail := s[len(s)-length:]
		if strings.HasPrefix(tag, tail) {
			return s[:len(s)-length], tail
		}
	}
	return s, ""
}
