package federation

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/stellar/go/address"
	strhttp "github.com/stellar/go/support/http"
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	q := r.URL.Query().Get("q")

	if q == "" {
		strhttp.WriteJSON(w, ErrorResponse{
			Code:    "invalid_request",
			Message: "q parameter is blank",
		}, http.StatusBadRequest)
		return
	}

	switch typ {
	case "name":
		h.lookupByName(w, q)
	case "id":
		h.lookupByID(w, q)
	case "txid":
		h.failNotImplemented(w, "txid type queries are not supported")
	default:
		strhttp.WriteJSON(w, ErrorResponse{
			Code:    "invalid_request",
			Message: fmt.Sprintf("invalid type: '%s'", typ),
		}, http.StatusBadRequest)
	}

}

func (h *Handler) failNotFound(w http.ResponseWriter) {
	strhttp.WriteJSON(w, ErrorResponse{
		Code:    "not_found",
		Message: "Account not found",
	}, http.StatusNotFound)
}

func (h *Handler) failNotImplemented(w http.ResponseWriter, msg string) {
	strhttp.WriteJSON(w, ErrorResponse{
		Code:    "not_implemented",
		Message: msg,
	}, http.StatusNotImplemented)
}

func (h *Handler) lookupByID(w http.ResponseWriter, q string) {
	rd, ok := h.Driver.(ReverseDriver)

	if !ok {
		h.failNotImplemented(w, "id type queries are not supported")
		return
	}

	// TODO: validate that `q` is a strkey encoded address

	rec, err := rd.LookupReverseRecord(q)
	if err != nil {
		strhttp.WriteError(w, errors.Wrap(err, "lookup record"))
		return
	}

	if rec == nil {
		h.failNotFound(w)
		return
	}

	strhttp.WriteJSON(w, SuccessResponse{
		StellarAddress: address.New(rec.Name, rec.Domain),
		AccountID:      q,
	}, http.StatusOK)
}

func (h *Handler) lookupByName(w http.ResponseWriter, q string) {
	name, domain, err := address.Split(q)
	if err != nil {
		strhttp.WriteJSON(w, ErrorResponse{
			Code:    "invalid_query",
			Message: "Please use an address of the form name*domain.com",
		}, http.StatusBadRequest)
		return
	}

	rec, err := h.Driver.LookupRecord(name, domain)
	if err != nil {
		strhttp.WriteError(w, errors.Wrap(err, "lookup record"))
		return
	}
	if rec == nil {
		h.failNotFound(w)
		return
	}

	strhttp.WriteJSON(w, SuccessResponse{
		StellarAddress: q,
		AccountID:      rec.AccountID,
		Memo:           rec.Memo,
		MemoType:       rec.MemoType,
	}, http.StatusOK)
}
