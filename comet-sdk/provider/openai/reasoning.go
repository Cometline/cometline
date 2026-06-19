package openai

import (
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

// embeddedReasoningTag pairs delimit chain-of-thought that some OpenAI-compatible
// providers embed inside the regular content stream instead of a dedicated field.
// Minimax M3 uses <think> when reasoning_split is disabled; Qwen
// and others commonly use XML-style think tags.
var embeddedReasoningTags = []struct {
	open, close string
}{
	{open: "<redacted_" + "thinking" + ">", close: "</redacted_" + "thinking" + ">"},
	{open: "<" + "think" + ">", close: "</" + "think" + ">"},
}

// contentReasoningSplitter extracts embedded thinking tags from streaming content
// deltas. It is safe across chunk boundaries where a tag may be split.
type contentReasoningSplitter struct {
	inReasoning bool
	active      int // index into embeddedReasoningTags, valid when inReasoning
	carry       strings.Builder
	reasoning   bool // whether ReasoningStartEvent was already emitted
}

func (s *contentReasoningSplitter) push(chunk string) []cometsdk.Event {
	if chunk == "" {
		return nil
	}
	s.carry.WriteString(chunk)
	return s.drain()
}

func (s *contentReasoningSplitter) drain() []cometsdk.Event {
	var events []cometsdk.Event
	for {
		text := s.carry.String()
		if text == "" {
			return events
		}
		s.carry.Reset()

		if !s.inReasoning {
			idx, tagIdx := findEarliestOpenTag(text)
			if idx < 0 {
				hold := holdbackTagPrefix(text, openTags())
				if hold > 0 {
					events = append(events, textDeltaEvents(text[:len(text)-hold])...)
					s.carry.WriteString(text[len(text)-hold:])
				} else {
					events = append(events, textDeltaEvents(text)...)
				}
				return events
			}
			if idx > 0 {
				events = append(events, textDeltaEvents(text[:idx])...)
			}
			tag := embeddedReasoningTags[tagIdx]
			text = text[idx+len(tag.open):]
			s.inReasoning = true
			s.active = tagIdx
			if !s.reasoning {
				events = append(events, cometsdk.ReasoningStartEvent{})
				s.reasoning = true
			}
			if text == "" {
				continue
			}
			s.carry.WriteString(text)
			continue
		}

		closeTag := embeddedReasoningTags[s.active].close
		idx := strings.Index(text, closeTag)
		if idx < 0 {
			hold := holdbackTagPrefix(text, []string{closeTag})
			if hold > 0 {
				events = append(events, reasoningDeltaEvents(text[:len(text)-hold])...)
				s.carry.WriteString(text[len(text)-hold:])
			} else {
				events = append(events, reasoningDeltaEvents(text)...)
			}
			return events
		}
		if idx > 0 {
			events = append(events, reasoningDeltaEvents(text[:idx])...)
		}
		text = text[idx+len(closeTag):]
		s.inReasoning = false
		s.active = -1
		if text == "" {
			continue
		}
		s.carry.WriteString(text)
	}
}

func findEarliestOpenTag(text string) (int, int) {
	bestIdx := -1
	bestTag := -1
	for i := range embeddedReasoningTags {
		tag := embeddedReasoningTags[i]
		if tag.open == "" {
			continue
		}
		idx := strings.Index(text, tag.open)
		if idx < 0 {
			continue
		}
		if bestIdx < 0 || idx < bestIdx {
			bestIdx = idx
			bestTag = i
		}
	}
	return bestIdx, bestTag
}

func openTags() []string {
	tags := make([]string, len(embeddedReasoningTags))
	for i, tag := range embeddedReasoningTags {
		tags[i] = tag.open
	}
	return tags
}

// holdbackTagPrefix returns how many trailing bytes of s might be the start of
// one of tags and must be buffered until the next chunk arrives.
func holdbackTagPrefix(s string, tags []string) int {
	max := 0
	for i := 1; i <= len(s); i++ {
		suffix := s[len(s)-i:]
		for _, tag := range tags {
			if suffix == tag {
				continue
			}
			if strings.HasPrefix(tag, suffix) && i > max {
				max = i
			}
		}
	}
	return max
}

func textDeltaEvents(text string) []cometsdk.Event {
	if text == "" {
		return nil
	}
	return []cometsdk.Event{cometsdk.TextDeltaEvent{Text: text}}
}

func reasoningDeltaEvents(text string) []cometsdk.Event {
	if text == "" {
		return nil
	}
	return []cometsdk.Event{cometsdk.ReasoningContentEvent{Text: text}}
}

// reasoningDetailsDelta returns the new reasoning text from a cumulative
// reasoning_details segment. MiniMax streams the full text-so-far in each chunk.
func reasoningDetailsDelta(prev, current string) string {
	if current == "" {
		return ""
	}
	if prev == "" {
		return current
	}
	if strings.HasPrefix(current, prev) {
		return current[len(prev):]
	}
	return current
}
