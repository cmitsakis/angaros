package broadcast

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	bolt "go.etcd.io/bbolt"

	"go.angaros.io/internal/dbutil"
)

type TimeRange struct {
	From time.Duration
	To   time.Duration
}

func (r TimeRange) String() string {
	return fmt.Sprintf("%v-%v", r.From.Hours(), r.To.Hours())
}

type TimeRanges []TimeRange

func (rs TimeRanges) String() string {
	rsStrings := make([]string, 0, len(rs))
	for _, r := range rs {
		rsStrings = append(rsStrings, r.String())
	}
	return strings.Join(rsStrings, " ")
}

type Broadcast struct {
	ID           ulid.ULID
	Contacts     []Contact
	MsgSubject   string `cbor:"MsgRawSubject"`
	MsgBody      string `cbor:"MsgRawBody"`
	MsgBodyFile  string `cbor:"Filename"`
	GatewayType  string
	GatewayKey   []byte
	SendDateFrom time.Time
	SendDateTo   time.Time
	SendHours    []TimeRange
	Timezone     string
	CreatedAt    time.Time
	status       string
}

func (b Broadcast) DBTable() string {
	return "broadcast"
}

func (b Broadcast) DBKey() []byte {
	return b.ID[:]
}

type broadcastsByStartableSince struct {
	Broadcasts []Broadcast
	SendHours  SettingSendHours
	Timezone   SettingTimezone
}

func (s broadcastsByStartableSince) Len() int {
	return len(s.Broadcasts)
}

func (s broadcastsByStartableSince) Swap(i, j int) {
	s.Broadcasts[i], s.Broadcasts[j] = s.Broadcasts[j], s.Broadcasts[i]
}

func (s broadcastsByStartableSince) Less(i, j int) bool {
	is := s.Broadcasts[i].startableNowUntil(s.SendHours, s.Timezone)
	js := s.Broadcasts[j].startableNowUntil(s.SendHours, s.Timezone)
	// treat zero time as infinity
	if is.IsZero() && !js.IsZero() {
		return false
	}
	if !is.IsZero() && js.IsZero() {
		return true
	}
	return is.Before(js)
}

func (b Broadcast) startableNowUntil(defaultSendHours SettingSendHours, defaultTimezone SettingTimezone) time.Time {
	var now time.Time
	if b.Timezone != "" {
		loc, err := time.LoadLocation(b.Timezone)
		if err != nil {
			// TODO: handle error
			return time.Time{}
		}
		now = time.Now().In(loc)
	} else if defaultTimezone != "" {
		// check default timezone from settings. if empty use local time
		loc, err := time.LoadLocation(string(defaultTimezone))
		if err != nil {
			// TODO: handle error
			return time.Time{}
		}
		now = time.Now().In(loc)
	} else {
		now = time.Now()
	}
	if !b.SendDateFrom.IsZero() && now.Before(b.SendDateFrom) {
		return time.Time{}
	}
	if !b.SendDateTo.IsZero() && now.After(b.SendDateTo.Add(24*time.Hour)) {
		return time.Time{}
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	timeOfDay := time.Since(today)

	var sendHours []TimeRange
	if len(b.SendHours) > 0 {
		sendHours = b.SendHours
	} else if len(defaultSendHours) > 0 {
		sendHours = defaultSendHours
	} else {
		return today.Add(24 * time.Hour)
	}
	for _, r := range sendHours {
		if timeOfDay >= r.From && timeOfDay < r.To {
			return today.Add(r.To)
		}
	}
	return time.Time{}
}

// startableAt is a modification of startableNowUntil(). TODO: combine them into one function
func (b Broadcast) startableAt(defaultSendHours SettingSendHours, defaultTimezone SettingTimezone) (time.Time, error) {
	var now time.Time
	if b.Timezone != "" {
		loc, err := time.LoadLocation(b.Timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to load location %s: %s", b.Timezone, err)
		}
		now = time.Now().In(loc)
	} else if defaultTimezone != "" {
		// check default timezone from settings. if empty use local time
		loc, err := time.LoadLocation(string(defaultTimezone))
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to load location %s: %s", string(defaultTimezone), err)
		}
		now = time.Now().In(loc)
	} else {
		now = time.Now()
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	timeOfDay := time.Since(today)

	if !b.SendDateTo.IsZero() && now.After(b.SendDateTo.Add(24*time.Hour)) {
		return time.Time{}, nil
	}

	var sendHours []TimeRange
	if len(b.SendHours) > 0 {
		sendHours = b.SendHours
	} else if len(defaultSendHours) > 0 {
		sendHours = defaultSendHours
	}

	for day := today; day.Before(day.Add(3 * 24 * time.Hour)); day = day.Add(24 * time.Hour) {
		if (b.SendDateFrom.IsZero() || day.After(b.SendDateFrom)) && (b.SendDateTo.IsZero() || day.Before(b.SendDateTo)) {
			if len(sendHours) == 0 {
				return day, nil
			}
			if day.Equal(today) {
				for _, r := range sendHours {
					if timeOfDay < r.To {
						return day.Add(r.From), nil
					}
				}
				continue
			} else {
				return day.Add(sendHours[0].From), nil
			}
		}
	}
	return time.Time{}, nil
}

func (b *Broadcast) getStartableInFromTx(tx *bolt.Tx) (*time.Duration, error) {
	// read settings
	var defaultSendHours SettingSendHours
	err := dbutil.GetByKeyTx(tx, defaultSendHours.DBKey(), &defaultSendHours)
	if err != nil && !errors.Is(err, dbutil.ErrNotFound) {
		return nil, fmt.Errorf("failed to read send hours settings from database: %s", err)
	}
	var defaultTimezone SettingTimezone
	err = dbutil.GetByKeyTx(tx, defaultTimezone.DBKey(), &defaultTimezone)
	if err != nil && !errors.Is(err, dbutil.ErrNotFound) {
		return nil, fmt.Errorf("failed to read time zone settings from database: %s", err)
	}
	startableAt, err := b.startableAt(defaultSendHours, defaultTimezone)
	if err != nil {
		return nil, fmt.Errorf("failed to read startable time: %s", err)
	}
	if startableAt.IsZero() {
		return nil, nil
	} else {
		d := time.Until(startableAt)
		return &d, nil
	}
}

func (b *Broadcast) ReadStatusFromTx(tx *bolt.Tx) error {
	var run Run
	err := dbutil.GetByKeyTx(tx, Run{BroadcastID: b.ID}.DBKey(), &run)
	if err != nil && !errors.Is(err, dbutil.ErrNotFound) {
		return fmt.Errorf("database error")
	}
	if errors.Is(err, dbutil.ErrNotFound) {
		b.status = "not started"
		startableIn, err := b.getStartableInFromTx(tx)
		if err != nil {
			return fmt.Errorf("failed to read startability: %s", err)
		}
		if startableIn == nil {
			b.status = "not startable"
		} else if *startableIn <= 0 {
			b.status = "starting now"
		} else {
			if *startableIn >= time.Minute {
				b.status = "starting in " + strings.TrimSuffix(startableIn.Round(time.Minute).String(), "0s")
			} else {
				b.status = "starting in " + startableIn.Round(time.Second).String()
			}
		}
		return nil
	} else {
		// lock mutex because we read from runningBroadcasts
		runningMutex.Lock()
		defer runningMutex.Unlock()
		if _, exists := runningBroadcasts[b.ID.String()]; exists {
			b.status = fmt.Sprintf("%d/%d sent - running", run.NextIndex, run.Length)
			return nil
		} else if run.NextIndex == run.Length {
			b.status = fmt.Sprintf("%d/%d sent - finished", run.NextIndex, run.Length)
			return nil
			//} else if !b.SendDateTo.IsZero() && now.After(b.SendDateTo.Add(24*time.Hour)) {
			//	b.status = fmt.Sprintf("%d/%d sent - expired", run.NextIndex, run.Length)
			//	return nil
		} else {
			b.status = fmt.Sprintf("%d/%d sent - paused", run.NextIndex, run.Length)
			return nil
		}
	}
}

func (b Broadcast) GetStatus() string {
	return b.status
}

func (b Broadcast) DetailsString(tx *bolt.Tx) (string, error) {
	var buf strings.Builder
	fmt.Fprintf(&buf, "ID: %s\n", b.ID.String())
	fmt.Fprintf(&buf, "Contacts: %d\n", len(b.Contacts))
	fmt.Fprintf(&buf, "Message subject: %s\n", b.MsgSubject)
	fmt.Fprintf(&buf, "Message body: %s\n", b.MsgBody)
	fmt.Fprintf(&buf, "Message body file: %s\n", b.MsgBodyFile)
	if err := b.ReadStatusFromTx(tx); err != nil {
		return "", fmt.Errorf("failed to get status: %s", err)
	}
	fmt.Fprintf(&buf, "Status: %s\n", b.GetStatus())
	fmt.Fprintf(&buf, "Gateway: %s\n", b.GatewayKey)
	fmt.Fprintf(&buf, "Send date from: %v\n", b.SendDateFrom)
	fmt.Fprintf(&buf, "Send date to: %v\n", b.SendDateTo)
	fmt.Fprintf(&buf, "Send time: %v\n", b.SendHours)
	return buf.String(), nil
}
