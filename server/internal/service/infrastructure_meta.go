package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/integrations/transwarp"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type MetainfoSaveResult struct {
	Saved   bool   `json:"saved"`
	AuditID string `json:"audit_id"`
}

func (s *InfrastructureService) saveMetainfo(ctx context.Context, connection db.InfrastructureConnection, client *transwarp.Client, actorID pgtype.UUID, actorType string, input InfrastructureInput) (MetainfoSaveResult, error) {
	if input.ServiceID <= 0 || input.Key == "" || input.Content == nil {
		return MetainfoSaveResult{}, errors.New("service_id, key and text content are required")
	}
	if strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 2000 {
		return MetainfoSaveResult{}, errors.New("a save reason of at most 2000 bytes is required")
	}
	current, err := client.ServiceMetaFile(ctx, input.ServiceID, input.Key)
	if err != nil {
		return MetainfoSaveResult{}, err
	}
	if !current.Text {
		return MetainfoSaveResult{}, errors.New("only text metainfo files can be edited")
	}
	auditID := dbid.NewV7()
	before, after := sha256.Sum256([]byte(current.Content)), sha256.Sum256([]byte(*input.Content))
	details, _ := json.Marshal(map[string]any{
		"audit_id": uuidString(auditID), "task_id": input.TaskID, "connection_id": uuidString(connection.ID), "service_id": input.ServiceID,
		"key": input.Key, "reason": input.Reason, "before_sha256": hex.EncodeToString(before[:]), "after_sha256": hex.EncodeToString(after[:]),
	})
	activityActor := actorType
	if activityActor == "user" {
		activityActor = "member"
	}
	record := func(recordCtx context.Context, action string) error {
		id := dbid.NewV7()
		if action == "infrastructure_metainfo_save_requested" {
			id = auditID
		}
		_, err := s.Queries.CreateActivity(recordCtx, db.CreateActivityParams{ID: id, WorkspaceID: connection.WorkspaceID, ActorType: pgtype.Text{String: activityActor, Valid: true}, ActorID: actorID, Action: action, Details: details})
		return err
	}
	if err := record(ctx, "infrastructure_metainfo_save_requested"); err != nil {
		return MetainfoSaveResult{}, errors.New("could not record metainfo save; no write was submitted")
	}
	err = client.SaveServiceMetaFile(ctx, input.ServiceID, input.Key, *input.Content)
	// A request cancellation must not erase the recorded outcome of an upstream write.
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	action := "infrastructure_metainfo_saved"
	if err != nil {
		action = "infrastructure_metainfo_save_unconfirmed"
	}
	if auditErr := record(auditCtx, action); auditErr != nil {
		return MetainfoSaveResult{}, fmt.Errorf("metainfo save %s was submitted but its outcome could not be recorded; read the file before retrying", uuidString(auditID))
	}
	if err != nil {
		return MetainfoSaveResult{}, fmt.Errorf("metainfo save %s is unconfirmed; read the file before retrying: %w", uuidString(auditID), err)
	}
	return MetainfoSaveResult{Saved: true, AuditID: uuidString(auditID)}, nil
}
