package model

import "fmt"

// Visibility is the publication state of a video.
type Visibility string

const (
	VisibilityPublic    Visibility = "public"
	VisibilityUnlisted  Visibility = "unlisted"
	VisibilityPrivate   Visibility = "private"
	VisibilityScheduled Visibility = "scheduled"
	VisibilityDraft     Visibility = "draft"
)

func (v Visibility) String() string {
	return string(v)
}

func ParseVisibility(s string) (Visibility, error) {
	v := Visibility(s)
	if v.IsValid() {
		return v, nil
	}
	return "", fmt.Errorf("invalid visibility: %q", s)
}

func (v Visibility) IsValid() bool {
	for _, item := range AllVisibilities() {
		if v == item {
			return true
		}
	}
	return false
}

func AllVisibilities() []Visibility {
	return []Visibility{
		VisibilityPublic,
		VisibilityUnlisted,
		VisibilityPrivate,
		VisibilityScheduled,
		VisibilityDraft,
	}
}

// Visibility All alias following the enum guideline naming.
func VisibilityAll() []Visibility {
	return AllVisibilities()
}

// SyncStatus is the state of a sync run.
type SyncStatus string

const (
	SyncStatusIdle    SyncStatus = "idle"
	SyncStatusRunning SyncStatus = "running"
	SyncStatusSuccess SyncStatus = "success"
	SyncStatusFailed  SyncStatus = "failed"
)

func (s SyncStatus) String() string {
	return string(s)
}

func ParseSyncStatus(s string) (SyncStatus, error) {
	v := SyncStatus(s)
	if v.IsValid() {
		return v, nil
	}
	return "", fmt.Errorf("invalid sync status: %q", s)
}

func (s SyncStatus) IsValid() bool {
	for _, item := range AllSyncStatuses() {
		if s == item {
			return true
		}
	}
	return false
}

func AllSyncStatuses() []SyncStatus {
	return []SyncStatus{
		SyncStatusIdle,
		SyncStatusRunning,
		SyncStatusSuccess,
		SyncStatusFailed,
	}
}

func SyncStatusAll() []SyncStatus {
	return AllSyncStatuses()
}

// Cadence is the publishing cadence for a series.
type Cadence string

const (
	CadenceDaily      Cadence = "daily"
	CadenceWeekly     Cadence = "weekly"
	CadenceTwoWeeks   Cadence = "two-weeks"
	CadenceThreeWeeks Cadence = "three-weeks"
	CadenceCustomDays Cadence = "custom-days"
)

func (c Cadence) String() string {
	return string(c)
}

func ParseCadence(s string) (Cadence, error) {
	v := Cadence(s)
	if v.IsValid() {
		return v, nil
	}
	return "", fmt.Errorf("invalid cadence: %q", s)
}

func (c Cadence) IsValid() bool {
	for _, item := range AllCadences() {
		if c == item {
			return true
		}
	}
	return false
}

func AllCadences() []Cadence {
	return []Cadence{
		CadenceDaily,
		CadenceWeekly,
		CadenceTwoWeeks,
		CadenceThreeWeeks,
		CadenceCustomDays,
	}
}

func CadenceAll() []Cadence {
	return AllCadences()
}

// Days returns the number of days for a cadence, or 0 for custom-days.
func (c Cadence) Days() int {
	switch c {
	case CadenceDaily:
		return 1
	case CadenceWeekly:
		return 7
	case CadenceTwoWeeks:
		return 14
	case CadenceThreeWeeks:
		return 21
	default:
		return 0
	}
}

// CommentRole is the role of a managed comment.
type CommentRole string

const (
	CommentRoleFirst    CommentRole = "first"
	CommentRolePinned   CommentRole = "pinned"
	CommentRoleStandard CommentRole = "standard"
)

func (r CommentRole) String() string {
	return string(r)
}

func ParseCommentRole(s string) (CommentRole, error) {
	v := CommentRole(s)
	if v.IsValid() {
		return v, nil
	}
	return "", fmt.Errorf("invalid comment role: %q", s)
}

func (r CommentRole) IsValid() bool {
	for _, item := range AllCommentRoles() {
		if r == item {
			return true
		}
	}
	return false
}

func AllCommentRoles() []CommentRole {
	return []CommentRole{
		CommentRoleFirst,
		CommentRolePinned,
		CommentRoleStandard,
	}
}

func CommentRoleAll() []CommentRole {
	return AllCommentRoles()
}

// ABTestStatus is the state of an A/B title test.
type ABTestStatus string

const (
	ABTestStatusDraft     ABTestStatus = "draft"
	ABTestStatusActive    ABTestStatus = "active"
	ABTestStatusCompleted ABTestStatus = "completed"
)

func (s ABTestStatus) String() string {
	return string(s)
}

func ParseABTestStatus(s string) (ABTestStatus, error) {
	v := ABTestStatus(s)
	if v.IsValid() {
		return v, nil
	}
	return "", fmt.Errorf("invalid ab test status: %q", s)
}

func (s ABTestStatus) IsValid() bool {
	for _, item := range AllABTestStatuses() {
		if s == item {
			return true
		}
	}
	return false
}

func AllABTestStatuses() []ABTestStatus {
	return []ABTestStatus{
		ABTestStatusDraft,
		ABTestStatusActive,
		ABTestStatusCompleted,
	}
}

func ABTestStatusAll() []ABTestStatus {
	return AllABTestStatuses()
}

// PeerDirection is the direction of a peer sync.
type PeerDirection string

const (
	PeerDirectionPush PeerDirection = "push"
	PeerDirectionPull PeerDirection = "pull"
)

func (d PeerDirection) String() string {
	return string(d)
}

func ParsePeerDirection(s string) (PeerDirection, error) {
	v := PeerDirection(s)
	if v.IsValid() {
		return v, nil
	}
	return "", fmt.Errorf("invalid peer direction: %q", s)
}

func (d PeerDirection) IsValid() bool {
	for _, item := range AllPeerDirections() {
		if d == item {
			return true
		}
	}
	return false
}

func AllPeerDirections() []PeerDirection {
	return []PeerDirection{
		PeerDirectionPush,
		PeerDirectionPull,
	}
}

func PeerDirectionAll() []PeerDirection {
	return AllPeerDirections()
}

// EmailDirection is the direction of a logged email.
type EmailDirection string

const (
	EmailDirectionSent     EmailDirection = "sent"
	EmailDirectionReceived EmailDirection = "received"
)

func (d EmailDirection) String() string {
	return string(d)
}

func ParseEmailDirection(s string) (EmailDirection, error) {
	v := EmailDirection(s)
	if v.IsValid() {
		return v, nil
	}
	return "", fmt.Errorf("invalid email direction: %q", s)
}

func (d EmailDirection) IsValid() bool {
	for _, item := range AllEmailDirections() {
		if d == item {
			return true
		}
	}
	return false
}

func AllEmailDirections() []EmailDirection {
	return []EmailDirection{
		EmailDirectionSent,
		EmailDirectionReceived,
	}
}

func EmailDirectionAll() []EmailDirection {
	return AllEmailDirections()
}

// All returns every Visibility value.
func (Visibility) All() []Visibility { return AllVisibilities() }

// All returns every SyncStatus value.
func (SyncStatus) All() []SyncStatus { return AllSyncStatuses() }

// All returns every Cadence value.
func (Cadence) All() []Cadence { return AllCadences() }

// All returns every CommentRole value.
func (CommentRole) All() []CommentRole { return AllCommentRoles() }

// All returns every ABTestStatus value.
func (ABTestStatus) All() []ABTestStatus { return AllABTestStatuses() }

// All returns every PeerDirection value.
func (PeerDirection) All() []PeerDirection { return AllPeerDirections() }

// All returns every EmailDirection value.
func (EmailDirection) All() []EmailDirection { return AllEmailDirections() }
