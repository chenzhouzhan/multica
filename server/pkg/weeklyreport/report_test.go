package weeklyreport

import (
	"testing"
)

func TestWeek(t *testing.T) {
	for _, value := range []string{"2026-11-10", "2026-02-30", "0000-01-01", "2026-1-5", ""} {
		if _, err := Week(value); err == nil {
			t.Errorf("accepted invalid week %q", value)
		}
	}
	if _, err := Week("2026-11-09"); err != nil {
		t.Fatal(err)
	}
}

func TestContentValidation(t *testing.T) {
	zero, negative := 0.0, -1.0
	draft := Content{Tasks: []Task{{Spent: &zero}}}
	if err := draft.Normalize(false); err != nil || draft.Tasks[0].ID == "" {
		t.Fatalf("incomplete draft: %+v %v", draft, err)
	}
	if err := draft.Normalize(true); err == nil {
		t.Fatal("published incomplete content")
	}
	draft.Tasks[0].Title, draft.Tasks[0].Completed = "Task", "Work completed"
	if err := draft.Normalize(true); err != nil || draft.Tasks[0].Remaining != nil {
		t.Fatalf("zero and unknown workloads must stay distinct: %v", err)
	}
	draft.Tasks[0].Spent = &negative
	if err := draft.Normalize(false); err == nil {
		t.Fatal("accepted negative workload")
	}
	draft.Tasks = []Task{{ID: "same"}, {ID: "same"}}
	if err := draft.Normalize(false); err == nil {
		t.Fatal("accepted duplicate row ids")
	}
}

func TestOverviewValidation(t *testing.T) {
	overview := Overview{VersionName: "Logits-3.0.0", VersionRows: []Progress{{Title: "Feature", LastProgress: "Connector ready; application integration pending"}}}
	if err := overview.Normalize(true); err != nil {
		t.Fatal(err)
	}
	if overview.VersionRows[0].ID == "" || overview.OnsiteRows == nil {
		t.Fatal("missing normalized fields")
	}
	overview.OnsiteRows = []Progress{{ID: overview.VersionRows[0].ID, Title: "Site"}}
	if err := overview.Normalize(false); err == nil {
		t.Fatal("accepted duplicate progress ids across groups")
	}
	if err := (&Overview{}).Normalize(true); err == nil {
		t.Fatal("generated empty overall progress")
	}
}
