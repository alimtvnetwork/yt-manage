package apiserver

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// abTestWithSnapshots builds a DTO for a test with its snapshots.
func (s *Server) abTestWithSnapshots(test model.ABTest) (abTestDTO, error) {
	snapshots, err := s.store.ABTests.ListSnapshots(test.ID)
	if err != nil {
		return abTestDTO{}, err
	}
	return toABTestDTO(test, snapshots), nil
}

// handleListABTests serves GET /api/ab-tests?videoId=ID, falling
// back to the active tests when videoId is absent.
func (s *Server) handleListABTests(w http.ResponseWriter, r *http.Request) {
	videoID, hasVideo, err := parseQueryUint(r, "videoId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var tests []model.ABTest
	if hasVideo {
		tests, err = s.store.ABTests.ListByVideo(videoID)
	} else {
		tests, err = s.store.ABTests.ListActive()
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]abTestDTO, 0, len(tests))
	for _, test := range tests {
		dto, err := s.abTestWithSnapshots(test)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		dtos = append(dtos, dto)
	}
	writeOK(w, dtos)
}

// createABTestRequest is the body of POST /api/ab-tests.
type createABTestRequest struct {
	VideoID  uint   `json:"videoId"`
	VariantA string `json:"variantA"`
	VariantB string `json:"variantB"`
}

// handleCreateABTest serves POST /api/ab-tests.
func (s *Server) handleCreateABTest(w http.ResponseWriter, r *http.Request) {
	var body createABTestRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.VideoID == 0 || body.VariantA == "" || body.VariantB == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("videoId, variantA and variantB are required"))
		return
	}
	test := &model.ABTest{
		VideoID: body.VideoID, VariantA: body.VariantA, VariantB: body.VariantB, IsActive: true,
	}
	if err := s.store.ABTests.Create(test); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, toABTestDTO(*test, nil))
}

// snapshotRequest is the body of POST /api/ab-tests/{id}/snapshot.
type snapshotRequest struct {
	CTRNote string `json:"ctrNote"`
}

// handleSnapshotABTest serves POST /api/ab-tests/{id}/snapshot: it
// records the video's current view count as a snapshot.
func (s *Server) handleSnapshotABTest(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body snapshotRequest
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	test, err := s.store.ABTests.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	video, err := s.store.Videos.GetByID(test.VideoID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	snapshot := &model.ABTestSnapshot{
		TestID: id, CapturedAt: time.Now(),
		ViewsAContext: strconv.FormatInt(video.ViewCount, 10), CTRNote: body.CTRNote,
	}
	if err := s.store.ABTests.AddSnapshot(snapshot); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, toSnapshotDTO(*snapshot))
}

// pickWinnerRequest is the body of POST /api/ab-tests/{id}/pick-winner.
type pickWinnerRequest struct {
	Winner string `json:"winner"`
}

// handlePickWinnerABTest serves POST /api/ab-tests/{id}/pick-winner:
// it applies the winning variant as the video title through the
// batch path and closes the test with that winner recorded.
func (s *Server) handlePickWinnerABTest(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body pickWinnerRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	test, err := s.store.ABTests.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	winningTitle := ""
	if body.Winner == "a" {
		winningTitle = test.VariantA
	} else if body.Winner == "b" {
		winningTitle = test.VariantB
	} else {
		writeError(w, http.StatusBadRequest, fmt.Errorf("winner must be \"a\" or \"b\""))
		return
	}
	video, err := s.store.Videos.GetByID(test.VideoID)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), video.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	plan := singleChangePlan(video, &winningTitle, nil)
	if _, err := batch.Apply(r.Context(), svc, s.store, plan); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	test.Winner = winningTitle
	test.IsActive = false
	if err := s.store.ABTests.Update(test); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dto, err := s.abTestWithSnapshots(*test)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, dto)
}
