package episodes

import "strings"

// ShowKey reduces a parsed series key (which is episode-level:
// it includes the episode's subtitle words) to the show-level
// key used by the series wizard, grouping hints, and watched
// series: the first two words of the key. Most shows carry a
// two-word name ("harbour walks", "mountain trails"); a
// one-word key stays one word. This is a heuristic by design -
// the parse layer deliberately keeps keys literal, and series
// detection groups on top of them.
func ShowKey(seriesKey string) string {
	words := strings.Fields(seriesKey)
	if len(words) <= 2 {
		return seriesKey
	}
	return strings.Join(words[:2], " ")
}

// EpisodeBaseKey strips the "|epN" suffix from a stored episode
// key, returning its series key.
func EpisodeBaseKey(episodeKey string) string {
	if idx := strings.LastIndex(episodeKey, "|ep"); idx >= 0 {
		return episodeKey[:idx]
	}
	return episodeKey
}
