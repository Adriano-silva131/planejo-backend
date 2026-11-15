package calendar

// Period is a part of the day: morning, afternoon or evening.
type Period string

const (
	Morning   Period = "morning"
	Afternoon Period = "afternoon"
	Evening   Period = "evening"
)

func (p Period) Valid() bool {
	switch p {
	case Morning, Afternoon, Evening:
		return true
	}
	return false
}

// PeriodOf returns the period t falls in: morning until noon, afternoon until
// 18:00 and evening after that.
func PeriodOf(t TimeOfDay) Period {
	switch {
	case t < 12*60:
		return Morning
	case t < 18*60:
		return Afternoon
	}
	return Evening
}
