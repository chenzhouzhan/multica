package weeklyreport

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxRows = 500
const MaxText = 10000

type Task struct {
	ID        string   `json:"id"`
	Version   string   `json:"version"`
	Title     string   `json:"title"`
	Completed string   `json:"completed"`
	Spent     *float64 `json:"spent"`
	Remaining *float64 `json:"remaining"`
	Note      string   `json:"note"`
}

type Content struct {
	Tasks []Task `json:"tasks"`
	Plan  string `json:"plan"`
}

type Progress struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	LastProgress string `json:"last_progress"`
	ThisPlan     string `json:"this_plan"`
	Note         string `json:"note"`
}

type Overview struct {
	VersionName string     `json:"version_name"`
	VersionRows []Progress `json:"version_rows"`
	OnsiteRows  []Progress `json:"onsite_rows"`
}

type Member struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type Contribution struct {
	Member      Member  `json:"member"`
	Content     Content `json:"content"`
	PublishedAt string  `json:"published_at"`
}

type Collection struct {
	Roster  []Member       `json:"roster"`
	Reports []Contribution `json:"reports"`
	Missing []Member       `json:"missing"`
}

type Snapshot struct {
	Overview   Overview   `json:"overview"`
	Collection Collection `json:"collection"`
}

func Week(value string) (time.Time, error) {
	day, err := time.Parse(time.DateOnly, value)
	if err != nil || day.Year() < 1970 || day.Weekday() != time.Monday {
		return time.Time{}, fmt.Errorf("week must be a Monday in YYYY-MM-DD format")
	}
	return day, nil
}

func normalizeID(id *string, seen map[string]bool) error {
	if *id == "" {
		*id = uuid.NewString()
	}
	if len(*id) > 80 || seen[*id] {
		return fmt.Errorf("row ids must be unique and at most 80 characters")
	}
	seen[*id] = true
	return nil
}

func textLimit(values ...string) error {
	for _, value := range values {
		if len([]rune(value)) > MaxText {
			return fmt.Errorf("text fields must be at most %d characters", MaxText)
		}
	}
	return nil
}

// Normalize accepts incomplete drafts; publishing checks the required content.
func (c *Content) Normalize(publishing bool) error {
	if len(c.Tasks) > MaxRows || publishing && len(c.Tasks) == 0 {
		return fmt.Errorf("a submitted weekly report needs 1 to %d tasks", MaxRows)
	}
	if c.Tasks == nil {
		c.Tasks = []Task{}
	}
	seen := map[string]bool{}
	for i := range c.Tasks {
		task := &c.Tasks[i]
		if err := normalizeID(&task.ID, seen); err != nil {
			return err
		}
		if publishing && (strings.TrimSpace(task.Title) == "" || strings.TrimSpace(task.Completed) == "") {
			return fmt.Errorf("task %d needs a title and completed content", i+1)
		}
		for _, value := range []*float64{task.Spent, task.Remaining} {
			if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0) {
				return fmt.Errorf("workload must be a non-negative finite number or null")
			}
		}
		if err := textLimit(task.Version, task.Title, task.Completed, task.Note); err != nil {
			return err
		}
	}
	return textLimit(c.Plan)
}

func (o *Overview) Normalize(generating bool) error {
	if len(o.VersionRows)+len(o.OnsiteRows) > MaxRows {
		return fmt.Errorf("overall progress supports at most %d rows", MaxRows)
	}
	if generating && (len(o.VersionRows)+len(o.OnsiteRows) == 0 || len(o.VersionRows) > 0 && strings.TrimSpace(o.VersionName) == "") {
		return fmt.Errorf("overall progress needs an item and a version name for version items")
	}
	if o.VersionRows == nil {
		o.VersionRows = []Progress{}
	}
	if o.OnsiteRows == nil {
		o.OnsiteRows = []Progress{}
	}
	seen := map[string]bool{}
	for _, rows := range [][]Progress{o.VersionRows, o.OnsiteRows} {
		for i := range rows {
			row := &rows[i]
			if err := normalizeID(&row.ID, seen); err != nil {
				return err
			}
			if generating && strings.TrimSpace(row.Title) == "" {
				return fmt.Errorf("every progress item needs task content")
			}
			if err := textLimit(row.Title, row.Type, row.LastProgress, row.ThisPlan, row.Note); err != nil {
				return err
			}
		}
	}
	return textLimit(o.VersionName)
}
