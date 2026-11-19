package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestImagePreviewIsBoundToItsActorTaskAndOriginator(t *testing.T) {
	agent := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	other := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	originator := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	task := "44444444-4444-4444-8444-444444444444"
	actor := ImageOperationActor{ID: agent, Type: "agent", TaskID: task, OriginatorID: originator}
	operation := db.InfrastructureOperation{ActorID: agent, ActorType: "agent"}
	plan := ImageReleasePlan{TaskID: task, OriginatorUserID: uuidString(originator)}
	for _, test := range []struct {
		name  string
		actor ImageOperationActor
		want  bool
	}{
		{"same task", actor, true},
		{"different agent", ImageOperationActor{ID: other, Type: "agent", TaskID: task, OriginatorID: originator}, false},
		{"different task", ImageOperationActor{ID: agent, Type: "agent", TaskID: uuidString(other), OriginatorID: originator}, false},
		{"different originator", ImageOperationActor{ID: agent, Type: "agent", TaskID: task, OriginatorID: other}, false},
		{"missing originator", ImageOperationActor{ID: agent, Type: "agent", TaskID: task}, false},
		{"missing task", ImageOperationActor{ID: agent, Type: "agent", OriginatorID: originator}, false},
		{"malformed task", ImageOperationActor{ID: agent, Type: "agent", TaskID: "invalid", OriginatorID: originator}, false},
		{"human with matching UUID", ImageOperationActor{ID: agent, Type: "user"}, false},
		{"missing actor", ImageOperationActor{ID: pgtype.UUID{}, Type: "agent", TaskID: task, OriginatorID: originator}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := imageActorMatches(operation, plan, test.actor); got != test.want {
				t.Fatalf("preview ownership = %v, want %v", got, test.want)
			}
		})
	}
	operation.ActorType = "user"
	if !imageActorMatches(operation, ImageReleasePlan{}, ImageOperationActor{ID: agent, Type: "user"}) {
		t.Fatal("existing human previews no longer work")
	}
	if imageActorMatches(operation, ImageReleasePlan{}, actor) {
		t.Fatal("an agent claimed a human preview")
	}
}
