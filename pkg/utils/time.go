package utils

import "time"

func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func NowUTC() time.Time {
	return time.Now().UTC()
}

func MinutesFromNow(m int) time.Time {
	return time.Now().UTC().Add(time.Duration(m) * time.Minute)
}

func HoursFromNow(h int) time.Time {
	return time.Now().UTC().Add(time.Duration(h) * time.Hour)
}

func DaysFromNow(d int) time.Time {
	return time.Now().UTC().Add(time.Duration(d) * 24 * time.Hour)
}

func FormatRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func IsExpired(t time.Time) bool {
	return time.Now().UTC().After(t)
}
