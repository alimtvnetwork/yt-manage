// Package episodes parses video titles into series, episode and
// part numbers and groups the parts of one episode together. The
// parser is pure (spec 03 §2): the same inconsistent titles a human
// reads as one episode — "Robo School EP3", "Robo School EP 03 -
// Part 2", "Robo School ep03 part three" — parse to one group.
package episodes

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Ordered title patterns (spec 03 §2). The episode patterns are
// tried in order and the first match wins; the part patterns follow
// the same rule.
var (
	episodePrimaryPattern = regexp.MustCompile(`(?i)\bep(?:isode)?\s*[-.]?\s*0*(\d+)`)
	episodeShortPattern   = regexp.MustCompile(`(?i)\be\s*0*(\d+)\b`)
	trailingNumberPattern = regexp.MustCompile(`(\d+)\s*$`)
	partNumberPattern     = regexp.MustCompile(`(?i)\bpart\s*[-.]?\s*0*(\d+)`)
	partWordPattern       = regexp.MustCompile(`(?i)\bpart\s+(one|two|three|four|five|six|seven|eight|nine|ten)\b`)
	partFractionPattern   = regexp.MustCompile(`(?i)\((\d+)\s*/\s*\d+\)`)
)

// partWordNumbers maps the part words one..ten to their numbers.
var partWordNumbers = map[string]int{
	"one":   1,
	"two":   2,
	"three": 3,
	"four":  4,
	"five":  5,
	"six":   6,
	"seven": 7,
	"eight": 8,
	"nine":  9,
	"ten":   10,
}

// ParsedTitle is the result of parsing one video title.
type ParsedTitle struct {
	SeriesKey     string
	EpisodeNumber int
	HasEpisode    bool
	PartNumber    int
	HasPart       bool
}

// ParseTitle extracts the series key, episode number and part
// number from a video title. A title with an episode number and no
// part marker is part 1 (the "EP3" case); a title with no episode
// number has HasEpisode false and PartNumber 0.
func ParseTitle(title string) ParsedTitle {
	parsed := ParsedTitle{}
	if episodeNumber, hasEpisode := parseEpisodeNumber(title); hasEpisode {
		parsed.EpisodeNumber = episodeNumber
		parsed.HasEpisode = true
	}
	if partNumber, hasPart := parsePartNumber(title); hasPart {
		parsed.PartNumber = partNumber
		parsed.HasPart = true
	}
	if parsed.HasEpisode && !parsed.HasPart {
		parsed.PartNumber = 1
	}
	if !parsed.HasEpisode {
		parsed.PartNumber = 0
	}
	parsed.SeriesKey = seriesKey(title)
	return parsed
}

// parseEpisodeNumber tries the episode patterns in their spec
// order: the ep/episode marker, then the bare "E" marker, then a
// trailing bare number.
func parseEpisodeNumber(title string) (int, bool) {
	if match := episodePrimaryPattern.FindStringSubmatch(title); match != nil {
		return parseNumber(match[1]), true
	}
	if match := episodeShortPattern.FindStringSubmatch(title); match != nil {
		return parseNumber(match[1]), true
	}
	if match := trailingNumberPattern.FindStringSubmatch(title); match != nil {
		return parseNumber(match[1]), true
	}
	return 0, false
}

// parsePartNumber tries the part patterns in their spec order: a
// digit marker, then a word marker (one..ten), then the "(n/m)"
// fraction form.
func parsePartNumber(title string) (int, bool) {
	if match := partNumberPattern.FindStringSubmatch(title); match != nil {
		return parseNumber(match[1]), true
	}
	if match := partWordPattern.FindStringSubmatch(title); match != nil {
		return partWordNumbers[strings.ToLower(match[1])], true
	}
	if match := partFractionPattern.FindStringSubmatch(title); match != nil {
		return parseNumber(match[1]), true
	}
	return 0, false
}

// parseNumber converts a regex digit capture to an int. The
// patterns only capture digits, so the only possible failure is an
// absurdly long digit run overflowing int; that yields 0.
func parseNumber(digits string) int {
	number, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return number
}

// stripMarkers removes every episode and part marker span from a
// title, replacing each match with a space. The part markers are
// stripped first: the trailing-number pattern would otherwise eat
// the digit of a trailing part marker ("... - Part 2"), and the
// episode patterns would otherwise leave its word behind.
func stripMarkers(title string) string {
	cleaned := partFractionPattern.ReplaceAllString(title, " ")
	cleaned = partWordPattern.ReplaceAllString(cleaned, " ")
	cleaned = partNumberPattern.ReplaceAllString(cleaned, " ")
	cleaned = episodePrimaryPattern.ReplaceAllString(cleaned, " ")
	cleaned = episodeShortPattern.ReplaceAllString(cleaned, " ")
	cleaned = trailingNumberPattern.ReplaceAllString(cleaned, " ")
	return cleaned
}

// seriesKey derives the grouping key of a title: the marker spans
// removed, all remaining digits removed, only letters and spaces
// kept, lowercased, with whitespace collapsed and trimmed.
func seriesKey(title string) string {
	cleaned := stripMarkers(title)
	var builder strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) {
			builder.WriteRune(unicode.ToLower(r))
		} else if unicode.IsSpace(r) {
			builder.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

// displayTitle derives the human-readable series name from an
// original video title: marker spans and digits stripped, original
// case kept, whitespace collapsed, and separators trimmed from the
// ends.
func displayTitle(title string) string {
	cleaned := stripMarkers(title)
	var builder strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) {
			builder.WriteRune(r)
		} else if unicode.IsSpace(r) {
			builder.WriteRune(' ')
		}
	}
	result := strings.Join(strings.Fields(builder.String()), " ")
	return strings.Trim(result, "-|: ")
}
