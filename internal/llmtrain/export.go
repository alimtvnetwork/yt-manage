// Package llmtrain exports training data for LLMs as JSONL: the
// static command documentation and database schema, plus
// anonymised usage shapes derived from the local mirror. The
// export never contains account data, tokens, email addresses,
// real titles, video ids, or channel ids.
package llmtrain

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// maxVideoShapes caps the number of video_shape records exported.
const maxVideoShapes = 1000

// commandRecord is the wire form of a command documentation entry.
type commandRecord struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Usage       string `json:"usage"`
	Description string `json:"description"`
}

// schemaRecord is the wire form of a table schema entry.
type schemaRecord struct {
	Type     string   `json:"type"`
	Database string   `json:"database"`
	Table    string   `json:"table"`
	Columns  []string `json:"columns"`
}

// episodeShape is the anonymised shape of one episode: only the
// word count of its series key, its number, its part count, and
// whether its part numbers have gaps.
type episodeShape struct {
	Type           string `json:"type"`
	SeriesKeyShape string `json:"seriesKeyShape"`
	EpisodeNumber  int    `json:"episodeNumber"`
	PartCount      int    `json:"partCount"`
	HasGaps        bool   `json:"hasGaps"`
}

// videoShape is the anonymised shape of one mirrored video:
// visibility, short flag, and a coarse duration bucket only.
type videoShape struct {
	Type           string `json:"type"`
	Visibility     string `json:"visibility"`
	IsShort        bool   `json:"isShort"`
	DurationBucket string `json:"durationBucket"`
}

// Export writes the JSONL training records to out and returns how
// many records were written. HTML escaping is disabled so values
// like "<word>" stay literal in the training data.
func Export(store *repo.Store, out io.Writer) (int, error) {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	count := 0
	write := func(record interface{}) error {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("write training record: %w", err)
		}
		count++
		return nil
	}
	for _, doc := range commandDocs {
		record := commandRecord{
			Type:        "command",
			Name:        doc.Name,
			Usage:       doc.Usage,
			Description: doc.Description,
		}
		if err := write(record); err != nil {
			return count, err
		}
	}
	for _, doc := range schemaDocs {
		record := schemaRecord{
			Type:     "schema",
			Database: doc.Database,
			Table:    doc.Table,
			Columns:  doc.Columns,
		}
		if err := write(record); err != nil {
			return count, err
		}
	}
	episodeShapes, err := buildEpisodeShapes(store)
	if err != nil {
		return count, err
	}
	for _, shape := range episodeShapes {
		if err := write(shape); err != nil {
			return count, err
		}
	}
	videoShapes, err := buildVideoShapes(store)
	if err != nil {
		return count, err
	}
	for _, shape := range videoShapes {
		if err := write(shape); err != nil {
			return count, err
		}
	}
	return count, nil
}

// buildEpisodeShapes derives one anonymised shape per stored
// episode across every channel.
func buildEpisodeShapes(store *repo.Store) ([]episodeShape, error) {
	channels, err := store.Channels.List()
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	shapes := []episodeShape{}
	for _, channel := range channels {
		episodes, err := store.Episodes.ListByChannel(channel.ID)
		if err != nil {
			return nil, fmt.Errorf("list episodes for channel %d: %w", channel.ID, err)
		}
		for _, episode := range episodes {
			parts, err := store.Episodes.ListParts(episode.ID)
			if err != nil {
				return nil, fmt.Errorf("list parts for episode %d: %w", episode.ID, err)
			}
			shapes = append(shapes, episodeShape{
				Type:           "episode_shape",
				SeriesKeyShape: seriesKeyShape(episode.EpisodeKey),
				EpisodeNumber:  episode.EpisodeNumber,
				PartCount:      len(parts),
				HasGaps:        hasPartGaps(parts),
			})
		}
	}
	return shapes, nil
}

// seriesKeyShape anonymises an episode key: the series portion
// (everything before the final "|ep<N>" suffix) keeps only its
// word count, with each word replaced by "<word>".
func seriesKeyShape(episodeKey string) string {
	series := episodeKey
	if idx := strings.LastIndex(series, "|"); idx >= 0 {
		series = series[:idx]
	}
	words := strings.Fields(series)
	shape := make([]string, len(words))
	for i := range words {
		shape[i] = "<word>"
	}
	return strings.Join(shape, " ")
}

// hasPartGaps reports whether the part numbers of an episode skip
// a number between the lowest and highest present part.
func hasPartGaps(parts []model.EpisodePart) bool {
	if len(parts) == 0 {
		return false
	}
	present := make(map[int]bool, len(parts))
	minPart := parts[0].PartNumber
	maxPart := parts[0].PartNumber
	for _, part := range parts {
		present[part.PartNumber] = true
		if part.PartNumber < minPart {
			minPart = part.PartNumber
		}
		if part.PartNumber > maxPart {
			maxPart = part.PartNumber
		}
	}
	for number := minPart; number <= maxPart; number++ {
		if !present[number] {
			return true
		}
	}
	return false
}

// buildVideoShapes derives one anonymised shape per mirrored
// video, capped at maxVideoShapes records.
func buildVideoShapes(store *repo.Store) ([]videoShape, error) {
	channels, err := store.Channels.List()
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	shapes := []videoShape{}
	for _, channel := range channels {
		if len(shapes) >= maxVideoShapes {
			break
		}
		videos, err := store.Videos.ListByChannel(channel.ID)
		if err != nil {
			return nil, fmt.Errorf("list videos for channel %d: %w", channel.ID, err)
		}
		for _, video := range videos {
			if len(shapes) >= maxVideoShapes {
				break
			}
			shapes = append(shapes, videoShape{
				Type:           "video_shape",
				Visibility:     video.Visibility.String(),
				IsShort:        video.IsShort,
				DurationBucket: durationBucket(video.DurationSeconds),
			})
		}
	}
	return shapes, nil
}

// durationBucket maps a duration in seconds to its coarse bucket.
func durationBucket(seconds int64) string {
	if seconds < 60 {
		return "<1m"
	}
	if seconds < 300 {
		return "1-5m"
	}
	if seconds < 900 {
		return "5-15m"
	}
	if seconds < 3600 {
		return "15-60m"
	}
	return ">60m"
}
