package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) InfrastructureServiceMeta(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	connection, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "connectionId"), "connection id")
	if !ok {
		return
	}
	client, err := h.Infrastructure.ServiceMetaClient(r.Context(), ws, connection)
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	serviceID, err := strconv.ParseInt(chi.URLParam(r, "serviceId"), 10, 64)
	if err != nil || serviceID <= 0 {
		writeError(w, 400, "a valid service id is required")
		return
	}
	key := r.URL.Query().Get("key")
	var result any
	if r.Method == http.MethodPut {
		var input struct {
			Content *string `json:"content"`
			Reason  string  `json:"reason"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&input); err != nil || input.Content == nil {
			writeError(w, 400, "text content is required")
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
		if input.Reason == "" {
			input.Reason = "Edit service metainfo"
		}
		result, err = h.Infrastructure.Execute(r.Context(), ws, connection, actor, "user", "save_metainfo_file", service.InfrastructureInput{ServiceID: serviceID, Key: key, Content: input.Content, Reason: input.Reason})
	} else if key != "" {
		result, err = client.ServiceMetaFile(r.Context(), serviceID, key)
	} else {
		result, err = client.ServiceMetaFiles(r.Context(), serviceID)
	}
	if err != nil {
		writeInfrastructureError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
