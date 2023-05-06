package api

import (
	"fmt"
	"github.com/amin1024/xtelbot/conf"
	"github.com/amin1024/xtelbot/core"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"html/template"
	"net/http"
	"strconv"
)

type RestHandler struct {
	router     *mux.Router
	subService *core.SubService
	log        *zap.SugaredLogger

	userService *core.UserService

	successfulCallback *template.Template
	failedCallback     *template.Template
}

func NewRestHandler(userService *core.UserService, callbackPath string) *RestHandler {
	log := conf.NewLogger()

	successfulCallback, err := template.ParseFiles("api/successful_purchase.html")
	if err != nil {
		log.Fatal(err)
	}
	failedCallback, err := template.ParseFiles("api/failed_purchase.html")
	if err != nil {
		log.Fatal(err)
	}
	router := mux.NewRouter()
	subService := core.NewSubService()
	handler := &RestHandler{
		router:             router,
		subService:         subService,
		userService:        userService,
		log:                log,
		successfulCallback: successfulCallback,
		failedCallback:     failedCallback,
	}
	router.HandleFunc("/v1/sub/{token}", handler.GetSub)

	router.HandleFunc("/"+callbackPath, handler.NextPayCallback)

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

// NextPayCallback handles users who have been redirected from bank by nextpay.org
func (h *RestHandler) NextPayCallback(rw http.ResponseWriter, req *http.Request) {
	oid, _ := strconv.ParseInt(req.URL.Query().Get("order_id"), 10, 64)
	amount, _ := strconv.ParseInt(req.URL.Query().Get("amount"), 10, 64)

	cp := core.CallbackParameters{
		TransactionId: req.URL.Query().Get("trans_id"),
		OrderId:       oid,
		Amount:        amount,
	}

	_, err := h.userService.VerifyBankTransaction(cp)
	if err != nil {
		if err := h.failedCallback.Execute(rw, cp); err != nil {
			h.log.Error(err)
		}
		return
	}
	if err := h.successfulCallback.Execute(rw, cp); err != nil {
		h.log.Error(err)
	}
}

func (h *RestHandler) Start(cert, key string) {
	h.log.Info("Starting REST router on ':443'")
	if err := http.ListenAndServeTLS(":443", cert, key, h.router); err != nil {
		h.log.Fatal(err)
	}
}
