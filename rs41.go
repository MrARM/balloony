package main

import (
	"errors"
	"time"
)

// RS41DateCodeTable maps the first letter of the serial to the manufacturing year.
var RS41DateCodeTable = map[byte]int{
	'A': 2028,
	'B': 2029,
	'C': 2030,
	'D': 2031,
	'E': 2032,
	'F': 2033,
	'G': 2034,
	'J': 2013,
	'K': 2014,
	'L': 2015,
	'M': 2016,
	'N': 2017,
	'P': 2018,
	'R': 2019,
	'S': 2020,
	'T': 2021,
	'U': 2022,
	'V': 2023,
	'W': 2024,
	'X': 2025,
	'Y': 2026,
	'Z': 2027,
}

// getDateOfISOWeek returns the time.Time for the Monday of the given ISO week and year.
func getDateOfISOWeek(week, year int) time.Time {
	// January 4th is always in week 1 according to ISO 8601
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	isoWeekStart := jan4.AddDate(0, 0, (week-1)*7)
	// Find the Monday of this week
	weekday := int(isoWeekStart.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday
	}
	return isoWeekStart.AddDate(0, 0, 1-weekday)
}

// ResolveRS41Date decodes the RS41 serial and returns the manufacturing date.
func ResolveRS41Date(serial string) (time.Time, error) {
	if len(serial) < 4 {
		return time.Time{}, errors.New("serial too short")
	}
	year, ok := RS41DateCodeTable[serial[0]]
	if !ok {
		return time.Time{}, errors.New("unknown year code")
	}
	week := 0
	for i := 1; i <= 2; i++ {
		week = week*10 + int(serial[i]-'0')
	}
	days := int(serial[3]-'0') - 1 // 0: Monday, 6: Sunday
	isoMonday := getDateOfISOWeek(week, year)
	return isoMonday.AddDate(0, 0, days), nil
}
