package model_test

import (
	"testing"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

func TestVisibilityRoundTrip(t *testing.T) {
	for _, v := range model.AllVisibilities() {
		if !v.IsValid() {
			t.Errorf("Visibility %q: IsValid() = false, want true", v)
		}
		got, err := model.ParseVisibility(v.String())
		if err != nil {
			t.Errorf("ParseVisibility(%q): unexpected error: %v", v.String(), err)
			continue
		}
		if got != v {
			t.Errorf("ParseVisibility(%q) = %q, want %q", v.String(), got, v)
		}
	}
	if len(model.AllVisibilities()) != 5 {
		t.Errorf("len(AllVisibilities()) = %d, want 5", len(model.AllVisibilities()))
	}
	if _, err := model.ParseVisibility("bogus"); err == nil {
		t.Error("ParseVisibility(\"bogus\"): expected error, got nil")
	}
	if model.Visibility("bogus").IsValid() {
		t.Error("Visibility(\"bogus\").IsValid() = true, want false")
	}
	if model.Visibility("").IsValid() {
		t.Error("zero Visibility IsValid() = true, want false")
	}
}

func TestSyncStatusRoundTrip(t *testing.T) {
	for _, v := range model.AllSyncStatuses() {
		if !v.IsValid() {
			t.Errorf("SyncStatus %q: IsValid() = false, want true", v)
		}
		got, err := model.ParseSyncStatus(v.String())
		if err != nil {
			t.Errorf("ParseSyncStatus(%q): unexpected error: %v", v.String(), err)
			continue
		}
		if got != v {
			t.Errorf("ParseSyncStatus(%q) = %q, want %q", v.String(), got, v)
		}
	}
	if len(model.AllSyncStatuses()) != 4 {
		t.Errorf("len(AllSyncStatuses()) = %d, want 4", len(model.AllSyncStatuses()))
	}
	if _, err := model.ParseSyncStatus("bogus"); err == nil {
		t.Error("ParseSyncStatus(\"bogus\"): expected error, got nil")
	}
	if model.SyncStatus("bogus").IsValid() {
		t.Error("SyncStatus(\"bogus\").IsValid() = true, want false")
	}
	if model.SyncStatus("").IsValid() {
		t.Error("zero SyncStatus IsValid() = true, want false")
	}
}

func TestCadenceRoundTrip(t *testing.T) {
	for _, v := range model.AllCadences() {
		if !v.IsValid() {
			t.Errorf("Cadence %q: IsValid() = false, want true", v)
		}
		got, err := model.ParseCadence(v.String())
		if err != nil {
			t.Errorf("ParseCadence(%q): unexpected error: %v", v.String(), err)
			continue
		}
		if got != v {
			t.Errorf("ParseCadence(%q) = %q, want %q", v.String(), got, v)
		}
	}
	if len(model.AllCadences()) != 5 {
		t.Errorf("len(AllCadences()) = %d, want 5", len(model.AllCadences()))
	}
	if _, err := model.ParseCadence("bogus"); err == nil {
		t.Error("ParseCadence(\"bogus\"): expected error, got nil")
	}
	if model.Cadence("bogus").IsValid() {
		t.Error("Cadence(\"bogus\").IsValid() = true, want false")
	}
	if model.Cadence("").IsValid() {
		t.Error("zero Cadence IsValid() = true, want false")
	}
}

func TestCadenceDays(t *testing.T) {
	cases := []struct {
		cadence model.Cadence
		want    int
	}{
		{model.CadenceDaily, 1},
		{model.CadenceWeekly, 7},
		{model.CadenceTwoWeeks, 14},
		{model.CadenceThreeWeeks, 21},
		{model.CadenceCustomDays, 0},
	}
	for _, tc := range cases {
		if got := tc.cadence.Days(); got != tc.want {
			t.Errorf("Cadence(%q).Days() = %d, want %d", tc.cadence, got, tc.want)
		}
	}
}

func TestCommentRoleRoundTrip(t *testing.T) {
	for _, v := range model.AllCommentRoles() {
		if !v.IsValid() {
			t.Errorf("CommentRole %q: IsValid() = false, want true", v)
		}
		got, err := model.ParseCommentRole(v.String())
		if err != nil {
			t.Errorf("ParseCommentRole(%q): unexpected error: %v", v.String(), err)
			continue
		}
		if got != v {
			t.Errorf("ParseCommentRole(%q) = %q, want %q", v.String(), got, v)
		}
	}
	if len(model.AllCommentRoles()) != 3 {
		t.Errorf("len(AllCommentRoles()) = %d, want 3", len(model.AllCommentRoles()))
	}
	if _, err := model.ParseCommentRole("bogus"); err == nil {
		t.Error("ParseCommentRole(\"bogus\"): expected error, got nil")
	}
	if model.CommentRole("bogus").IsValid() {
		t.Error("CommentRole(\"bogus\").IsValid() = true, want false")
	}
	if model.CommentRole("").IsValid() {
		t.Error("zero CommentRole IsValid() = true, want false")
	}
}

func TestABTestStatusRoundTrip(t *testing.T) {
	for _, v := range model.AllABTestStatuses() {
		if !v.IsValid() {
			t.Errorf("ABTestStatus %q: IsValid() = false, want true", v)
		}
		got, err := model.ParseABTestStatus(v.String())
		if err != nil {
			t.Errorf("ParseABTestStatus(%q): unexpected error: %v", v.String(), err)
			continue
		}
		if got != v {
			t.Errorf("ParseABTestStatus(%q) = %q, want %q", v.String(), got, v)
		}
	}
	if len(model.AllABTestStatuses()) != 3 {
		t.Errorf("len(AllABTestStatuses()) = %d, want 3", len(model.AllABTestStatuses()))
	}
	if _, err := model.ParseABTestStatus("bogus"); err == nil {
		t.Error("ParseABTestStatus(\"bogus\"): expected error, got nil")
	}
	if model.ABTestStatus("bogus").IsValid() {
		t.Error("ABTestStatus(\"bogus\").IsValid() = true, want false")
	}
	if model.ABTestStatus("").IsValid() {
		t.Error("zero ABTestStatus IsValid() = true, want false")
	}
}

func TestPeerDirectionRoundTrip(t *testing.T) {
	for _, v := range model.AllPeerDirections() {
		if !v.IsValid() {
			t.Errorf("PeerDirection %q: IsValid() = false, want true", v)
		}
		got, err := model.ParsePeerDirection(v.String())
		if err != nil {
			t.Errorf("ParsePeerDirection(%q): unexpected error: %v", v.String(), err)
			continue
		}
		if got != v {
			t.Errorf("ParsePeerDirection(%q) = %q, want %q", v.String(), got, v)
		}
	}
	if len(model.AllPeerDirections()) != 2 {
		t.Errorf("len(AllPeerDirections()) = %d, want 2", len(model.AllPeerDirections()))
	}
	if _, err := model.ParsePeerDirection("bogus"); err == nil {
		t.Error("ParsePeerDirection(\"bogus\"): expected error, got nil")
	}
	if model.PeerDirection("bogus").IsValid() {
		t.Error("PeerDirection(\"bogus\").IsValid() = true, want false")
	}
	if model.PeerDirection("").IsValid() {
		t.Error("zero PeerDirection IsValid() = true, want false")
	}
}

func TestEmailDirectionRoundTrip(t *testing.T) {
	for _, v := range model.AllEmailDirections() {
		if !v.IsValid() {
			t.Errorf("EmailDirection %q: IsValid() = false, want true", v)
		}
		got, err := model.ParseEmailDirection(v.String())
		if err != nil {
			t.Errorf("ParseEmailDirection(%q): unexpected error: %v", v.String(), err)
			continue
		}
		if got != v {
			t.Errorf("ParseEmailDirection(%q) = %q, want %q", v.String(), got, v)
		}
	}
	if len(model.AllEmailDirections()) != 2 {
		t.Errorf("len(AllEmailDirections()) = %d, want 2", len(model.AllEmailDirections()))
	}
	if _, err := model.ParseEmailDirection("bogus"); err == nil {
		t.Error("ParseEmailDirection(\"bogus\"): expected error, got nil")
	}
	if model.EmailDirection("bogus").IsValid() {
		t.Error("EmailDirection(\"bogus\").IsValid() = true, want false")
	}
	if model.EmailDirection("").IsValid() {
		t.Error("zero EmailDirection IsValid() = true, want false")
	}
}
