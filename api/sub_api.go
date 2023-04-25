package api

import (
	"fmt"
	"github.com/amin1024/xtelbot/conf"
	"github.com/amin1024/xtelbot/core"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
)

type RestHandler struct {
	router     *mux.Router
	subService *core.SubService
	log        *zap.SugaredLogger
}

func NewRestHandler() *RestHandler {
	router := mux.NewRouter()
	subService := core.NewSubService()
	handler := &RestHandler{
		router:     router,
		subService: subService,
		log:        conf.NewLogger(),
	}
	router.HandleFunc("/v1/sub/{token}", handler.GetSub)

	return handler
}

func (h *RestHandler) GetSub(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	token := vars["token"]
	subsContent, err := h.subService.GenerateUserSub(token)
	if err != nil {
		// TODO: user shouldn't receive this shit!
		h.log.Error("error while generating subscription content:", err)
		fmt.Fprint(rw, "user not found!\n")
	}
	fmt.Fprintf(rw, subsContent)
}

func (h *RestHandler) Start(cert, key string) {
	h.log.Info("Starting REST router on ':443'")
	if err := http.ListenAndServeTLS(":443", cert, key, h.router); err != nil {
		h.log.Fatal(err)
	}
}
