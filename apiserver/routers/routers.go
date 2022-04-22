package routers

import (
	"io"
	"net/http"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"runner-manager/apiserver/controllers"
)

func NewAPIRouter(han *controllers.APIController, logWriter io.Writer) *mux.Router {
	router := mux.NewRouter()
	log := gorillaHandlers.CombinedLoggingHandler
	apiRouter := router.PathPrefix("").Subrouter()

	apiRouter.PathPrefix("/").Handler(log(logWriter, http.HandlerFunc(han.CatchAll)))

	return router
}
