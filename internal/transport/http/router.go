package transporthttp

import (
	"net/http"

	"github.com/gorilla/mux"

	swaggerdocs "example.com/taskservice/internal/transport/http/docs"
	httphandlers "example.com/taskservice/internal/transport/http/handlers"
)

func NewRouter(
	taskHandler *httphandlers.TaskHandler,
	scheduleHandler *httphandlers.ScheduleHandler,
	docsHandler *swaggerdocs.Handler,
) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/swagger/openapi.json", docsHandler.ServeSpec).Methods(http.MethodGet)
	router.HandleFunc("/swagger/", docsHandler.ServeUI).Methods(http.MethodGet)
	router.HandleFunc("/swagger", docsHandler.RedirectToUI).Methods(http.MethodGet)

	api := router.PathPrefix("/api/v1").Subrouter()

	// Task endpoints
	api.HandleFunc("/tasks", taskHandler.Create).Methods(http.MethodPost)
	api.HandleFunc("/tasks", taskHandler.List).Methods(http.MethodGet)
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.GetByID).Methods(http.MethodGet)
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.Update).Methods(http.MethodPut)
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.Delete).Methods(http.MethodDelete)

	// Schedule endpoints
	api.HandleFunc("/schedules", scheduleHandler.Create).Methods(http.MethodPost)
	api.HandleFunc("/schedules", scheduleHandler.List).Methods(http.MethodGet)
	api.HandleFunc("/schedules/{id:[0-9]+}", scheduleHandler.GetByID).Methods(http.MethodGet)
	api.HandleFunc("/schedules/{id:[0-9]+}", scheduleHandler.Update).Methods(http.MethodPut)
	api.HandleFunc("/schedules/{id:[0-9]+}", scheduleHandler.Delete).Methods(http.MethodDelete)
	api.HandleFunc("/schedules/{id:[0-9]+}/activate", scheduleHandler.Activate).Methods(http.MethodPost)
	api.HandleFunc("/schedules/{id:[0-9]+}/deactivate", scheduleHandler.Deactivate).Methods(http.MethodPost)

	return router
}
