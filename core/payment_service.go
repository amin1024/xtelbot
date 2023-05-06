package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type PaymentTerminal interface {
	CreateToken(int64, int64) (string, error)
	VerifyOrder(CallbackParameters) (VerifyResponse, error)
	CreateRedirectUrl(transId string) string
}

type CallbackParameters struct {
	TransactionId string
	OrderId       int64
	Amount        int64
}

type VerifyResponse struct {
	Code        int64  `json:"code"`
	Amount      int64  `json:"amount"`
	OrderId     string `json:"order_id"`
	CardHolder  string `json:"card_holder"`
	ShaparakRef string `json:"Shaparak_Ref_Id"`
	Custom      interface{}
}

// -----------------------------------------------------------------

type NextPayTerminal struct {
	apiKey      string
	callbackUri string

	tokenUri    string
	verifyUri   string
	redirectUri string
}

func NewNextPayTerminal(apiKey, callbackUri string) NextPayTerminal {
	return NextPayTerminal{
		apiKey:      apiKey,
		callbackUri: callbackUri,
		tokenUri:    "https://nextpay.org/nx/gateway/token",
		verifyUri:   "https://nextpay.org/nx/gateway/verify",
		redirectUri: "https://nextpay.org/nx/gateway/payment/%s",
	}
}

func (n NextPayTerminal) CreateToken(amount, purchaseId int64) (string, error) {
	values := make(url.Values)
	values.Set("api_key", n.apiKey)
	values.Add("amount", strconv.FormatInt(amount, 10))
	values.Add("order_id", strconv.FormatInt(purchaseId, 10))
	values.Add("callback_uri", n.callbackUri)

	response, err := http.PostForm(n.tokenUri, values)
	if err != nil {
		return "", err
	}
	marshallResponse := struct {
		Code          int64  `json:"code"`
		TransactionId string `json:"trans_id"`
	}{}
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(b, &marshallResponse); err != nil {
		return "", err
	}

	if marshallResponse.Code != -1 {
		return "", fmt.Errorf("nextPay error code %d", marshallResponse.Code)
	}
	return marshallResponse.TransactionId, nil
}

func (n NextPayTerminal) VerifyOrder(parameters CallbackParameters) (VerifyResponse, error) {
	vr := VerifyResponse{}
	values := make(url.Values)
	values.Set("api_key", n.apiKey)
	values.Add("trans_id", parameters.TransactionId)
	values.Add("amount", strconv.FormatInt(parameters.Amount, 10))

	response, err := http.PostForm(n.verifyUri, values)
	if err != nil {
		return vr, err
	}
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return vr, err
	}

	if err := json.Unmarshal(b, &vr); err != nil {
		return vr, err
	}

	return vr, nil
}

func (n NextPayTerminal) CreateRedirectUrl(transId string) string {
	return fmt.Sprintf(n.redirectUri, transId)
}
