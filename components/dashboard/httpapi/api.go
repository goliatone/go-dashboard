package httpapi

import (
	"encoding/json"
	"net/http"

	gocommand "github.com/goliatone/go-command"
	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
)

// Handlers exposes HTTP endpoints backed by shared commands.
type Handlers struct {
	Assign  gocommand.Commander[dashboard.AddWidgetRequest]
	Remove  gocommand.Commander[commands.RemoveWidgetInput]
	Reorder gocommand.Commander[commands.ReorderWidgetsInput]
	Refresh gocommand.Commander[commands.RefreshWidgetInput]
}

func (h *Handlers) HandleAssignWidget(w http.ResponseWriter, r *http.Request) {
	var payload dashboard.AddWidgetRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.Assign.Execute(r.Context(), payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handlers) HandleRemoveWidget(w http.ResponseWriter, r *http.Request, widgetID string) {
	input := commands.RemoveWidgetInput{WidgetID: widgetID}
	if err := h.Remove.Execute(r.Context(), input); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) HandleReorderWidgets(w http.ResponseWriter, r *http.Request) {
	var payload commands.ReorderWidgetsInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.Reorder.Execute(r.Context(), payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) HandleRefreshWidget(w http.ResponseWriter, r *http.Request) {
	var payload commands.RefreshWidgetInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.Refresh.Execute(r.Context(), payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
