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
