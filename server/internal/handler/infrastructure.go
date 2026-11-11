package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/integrations/transwarp"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/infrastructurecontract"
)

func (h *Handler) ListInfrastructureConnections(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	rows, err := h.Infrastructure.List(r.Context(), ws)
	if err != nil {
		writeError(w, 500, "could not list Manager connections")
		return
	}
	member, _ := middleware.MemberFromContext(r.Context())
	writeJSON(w, 200, map[string]any{"connections": rows, "configured": h.Infrastructure.Secrets != nil, "can_manage": roleAllowed(member.Role, "owner", "admin") && !isMachineCredentialActor(r)})
}

func (h *Handler) SaveInfrastructureConnection(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	var id pgtype.UUID
	if value := chi.URLParam(r, "connectionId"); value != "" {
		id, ok = parseUUIDOrBadRequest(w, value, "connection id")
		if !ok {
			return
		}
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	actor, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return
	}
	if h.Infrastructure.Secrets == nil {
		writeError(w, 503, "configure MULTICA_INFRASTRUCTURE_SECRET_KEY before adding Managers")
		return
	}
	var input service.InfrastructureConnectionInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, 400, "invalid connection input")
		return
	}
	result, err := h.Infrastructure.Save(r.Context(), ws, id, actor, input)
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) DeleteInfrastructureConnection(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "connectionId"), "connection id")
	if !ok {
		return
	}
	if err := h.Infrastructure.Delete(r.Context(), ws, id); err != nil {
		writeInfrastructureError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) SetInfrastructureGrants(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "connectionId"), "connection id")
	if !ok {
		return
	}
	var input struct {
		Grants []service.InfrastructureGrant `json:"grants"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, 400, "invalid grant input")
		return
	}
	result, err := h.Infrastructure.SetGrants(r.Context(), ws, id, input.Grants)
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func writeInfrastructureError(w http.ResponseWriter, err error) {
	if errors.Is(err, transwarp.ErrResponse) {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "Manager connection or operation not found")
		return
	}
	writeError(w, 400, err.Error())
}

func (h *Handler) InvokeInfrastructureTool(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	if isMachineCredentialActor(r) {
		if r.Header.Get("X-Actor-Source") != "task_token" {
			writeError(w, 403, "infrastructure tools require a human or a running task credential")
			return
		}
		taskID, ok := parseUUIDOrBadRequest(w, r.Header.Get("X-Task-ID"), "task id")
		if !ok {
			return
		}
		task, err := h.Queries.GetAgentTask(r.Context(), taskID)
		if err != nil || uuidToString(task.AgentID) != r.Header.Get("X-Agent-ID") {
			writeError(w, 403, "task credential does not match its agent")
			return
		}
		scope, err := h.TaskService.ResolveTaskScopeChecked(r.Context(), task)
		if err != nil || scope.WorkspaceID != uuidToString(ws) {
			writeError(w, 403, "task does not belong to this workspace")
			return
		}
		if task.Status != "running" {
			writeError(w, 409, "task is not running")
			return
		}
		h.executeInfrastructure(w, r, ws, task.AgentID, task.OriginatorUserID, "agent")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	actor, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return
	}
	h.executeInfrastructure(w, r, ws, actor, actor, "user")
}

func (h *Handler) InvokeAgentInfrastructureTool(w http.ResponseWriter, r *http.Request) {
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if task.Status != "running" {
		writeError(w, 409, "task is not running")
		return
	}
	h.executeInfrastructure(w, r, parseUUID(workspaceID), task.AgentID, task.OriginatorUserID, "agent")
}

func (h *Handler) executeInfrastructure(w http.ResponseWriter, r *http.Request, ws, actor, principal pgtype.UUID, actorType string) {
	tool := chi.URLParam(r, "tool")
	if !infrastructurecontract.Known(tool) {
		writeError(w, 404, "unknown infrastructure tool")
		return
	}
	if tool == "save_metainfo_file" {
		if !principal.Valid {
			writeError(w, 403, "metainfo edits require a workspace owner/admin human originator")
			return
		}
		member, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{UserID: principal, WorkspaceID: ws})
		if err != nil || !roleAllowed(member.Role, "owner", "admin") {
			writeError(w, 403, "metainfo edits require a workspace owner/admin human originator")
			return
		}
	}
	if tool == "list_managers" {
		result, err := h.Infrastructure.List(r.Context(), ws)
		if err != nil {
			writeInfrastructureError(w, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	var input service.InfrastructureInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&input); err != nil {
		writeError(w, 400, "invalid tool input")
		return
	}
	conn, ok := parseUUIDOrBadRequest(w, input.ConnectionID, "connection_id")
	if !ok {
		return
	}
	if actorType == "agent" {
		input.TaskID = r.Header.Get("X-Task-ID")
		if input.TaskID == "" {
			input.TaskID = chi.URLParam(r, "id")
		}
	}
	var result any
	var err error
	if tool == "get_operation" {
		id, ok := parseUUIDOrBadRequest(w, input.OperationID, "operation_id")
		if !ok {
			return
		}
		result, err = h.Infrastructure.RefreshOperation(r.Context(), ws, conn, id)
	} else {
		result, err = h.Infrastructure.Execute(r.Context(), ws, conn, actor, actorType, tool, input)
	}
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
