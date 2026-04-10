package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	scheduledomain "example.com/taskservice/internal/domain/schedule"
	scheduleusecase "example.com/taskservice/internal/usecase/schedule"
)

// ScheduleHandler handles HTTP requests for the schedule resource.
type ScheduleHandler struct {
	usecase scheduleusecase.Usecase
}

// NewScheduleHandler constructs a ScheduleHandler.
func NewScheduleHandler(usecase scheduleusecase.Usecase) *ScheduleHandler {
	return &ScheduleHandler{usecase: usecase}
}

func (h *ScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req scheduleRequestDTO
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	input, err := req.toCreateInput()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	sched, err := h.usecase.Create(r.Context(), input)
	if err != nil {
		writeScheduleUsecaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, newScheduleDTO(sched))
}

func (h *ScheduleHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := getScheduleIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	sched, err := h.usecase.GetByID(r.Context(), id)
	if err != nil {
		writeScheduleUsecaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, newScheduleDTO(sched))
}

func (h *ScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := getScheduleIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req scheduleRequestDTO
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	input, err := req.toUpdateInput()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	sched, err := h.usecase.Update(r.Context(), id, input)
	if err != nil {
		writeScheduleUsecaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, newScheduleDTO(sched))
}

func (h *ScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := getScheduleIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := h.usecase.Delete(r.Context(), id); err != nil {
		writeScheduleUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.usecase.List(r.Context())
	if err != nil {
		writeScheduleUsecaseError(w, err)
		return
	}

	response := make([]scheduleDTO, 0, len(schedules))
	for i := range schedules {
		response = append(response, newScheduleDTO(&schedules[i]))
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *ScheduleHandler) Activate(w http.ResponseWriter, r *http.Request) {
	id, err := getScheduleIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := h.usecase.Activate(r.Context(), id); err != nil {
		writeScheduleUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ScheduleHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := getScheduleIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := h.usecase.Deactivate(r.Context(), id); err != nil {
		writeScheduleUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getScheduleIDFromRequest(r *http.Request) (int64, error) {
	rawID := mux.Vars(r)["id"]
	if rawID == "" {
		return 0, errors.New("missing schedule id")
	}

	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid schedule id")
	}

	return id, nil
}

func writeScheduleUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, scheduledomain.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, scheduleusecase.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}
