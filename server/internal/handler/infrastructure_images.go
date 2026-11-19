package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func infrastructureImageScope(w http.ResponseWriter, r *http.Request, withService bool) (pgtype.UUID, pgtype.UUID, int64, int64, bool) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return ws, pgtype.UUID{}, 0, 0, false
	}
	connection, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "connectionId"), "connection id")
	if !ok {
		return ws, connection, 0, 0, false
	}
	cluster, err := strconv.ParseInt(chi.URLParam(r, "clusterId"), 10, 64)
	if err != nil || cluster <= 0 {
		writeError(w, http.StatusBadRequest, "a positive cluster id is required")
		return ws, connection, 0, 0, false
	}
	var serviceID int64
	if withService {
		serviceID, err = strconv.ParseInt(chi.URLParam(r, "serviceId"), 10, 64)
		if err != nil || serviceID <= 0 {
			writeError(w, http.StatusBadRequest, "a positive service id is required")
			return ws, connection, cluster, 0, false
		}
	}
	return ws, connection, cluster, serviceID, true
}

func (h *Handler) requireInfrastructureImageAdmin(w http.ResponseWriter, r *http.Request, workspace pgtype.UUID) (pgtype.UUID, bool) {
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "image publication and execution settings require a workspace owner/admin human")
		return pgtype.UUID{}, false
	}
	user, ok := requireUserID(w, r)
	if !ok {
		return pgtype.UUID{}, false
	}
	actor, ok := parseUUIDOrBadRequest(w, user, "user id")
	if !ok {
		return actor, false
	}
	member, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{UserID: actor, WorkspaceID: workspace})
	if err != nil || !roleAllowed(member.Role, "owner", "admin") {
		writeError(w, http.StatusForbidden, "image publication and execution settings require a workspace owner/admin")
		return actor, false
	}
	return actor, true
}

func (h *Handler) InfrastructureExecutionAccess(w http.ResponseWriter, r *http.Request) {
	workspace, connection, cluster, _, ok := infrastructureImageScope(w, r, false)
	if !ok {
		return
	}
	if _, ok := h.requireInfrastructureImageAdmin(w, r, workspace); !ok {
		return
	}
	var result service.ExecutionAccessInfo
	var err error
	if r.Method == http.MethodPut {
		var input service.ExecutionAccessInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid SSH execution access")
			return
		}
		result, err = h.Infrastructure.SaveExecutionAccess(r.Context(), workspace, connection, cluster, input)
	} else {
		result, err = h.Infrastructure.GetExecutionAccess(r.Context(), workspace, connection, cluster)
	}
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) InfrastructureImageResources(w http.ResponseWriter, r *http.Request) {
	workspace, connection, cluster, serviceID, ok := infrastructureImageScope(w, r, true)
	if !ok {
		return
	}
	result, err := h.Infrastructure.ImageResources(r.Context(), workspace, connection, cluster, serviceID)
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) PreviewInfrastructureImages(w http.ResponseWriter, r *http.Request) {
	workspace, connection, cluster, serviceID, ok := infrastructureImageScope(w, r, true)
	if !ok {
		return
	}
	actor, ok := h.requireInfrastructureImageAdmin(w, r, workspace)
	if !ok {
		return
	}
	var input service.ImagePreviewInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid image preview input")
		return
	}
	result, err := h.Infrastructure.PreviewImages(r.Context(), workspace, connection, service.ImageOperationActor{ID: actor, Type: "user"}, cluster, serviceID, input)
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (h *Handler) StartInfrastructureImages(w http.ResponseWriter, r *http.Request) {
	workspace, connection, cluster, serviceID, ok := infrastructureImageScope(w, r, true)
	if !ok {
		return
	}
	actor, ok := h.requireInfrastructureImageAdmin(w, r, workspace)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "operationId"), "operation id")
	if !ok {
		return
	}
	result, err := h.Infrastructure.StartImageOperation(r.Context(), workspace, connection, service.ImageOperationActor{ID: actor, Type: "user"}, id, cluster, serviceID)
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}
