package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)


const (
	OkToken               = "OkToken"
	DoBigTimeout          = "DoBigTimeout"
	DoFatalError          = "DoFatalError"
	DoBadRequest          = "DoBadRequest"
	DoSearchErrorResponse = "DoSearchErrorResponse"
	DoErrorBadOrderField  = "ErrorBadOrderField"
	DoWrongJson           = "DoWrongJson"
)

//Names of allowed fields to order
var allowedField = []string{
	"Id",
	"Age",
	"Name",
}

//Indexes of allowedField
const (
	fieldIdIndex = iota
	fieldAgeIndex
	fieldNameIndex
)

//logging an error
func logE(err error) {
	fmt.Println(err)
}

//serveAuthorization checks an access token
//return true if 'AccessToken' is ok
func serveAuthorization(w http.ResponseWriter, r *http.Request) (bool, error) {
	if r.Header.Get("AccessToken") != OkToken {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write(nil)
		if err != nil {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

//serveTimeout checks and emulates a big timeout
//return true if 2-seconds timeout has been emulated
func serveTimeout(query string) bool {
	if query == DoBigTimeout {
		time.Sleep(time.Second * 2)
		return true
	}
	return false
}

//serveInternalError sends http.StatusInternalServerError
//return true if succeeded
func serveInternalError(w http.ResponseWriter, query string) (bool, error) {
	if query == DoFatalError {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write(nil)
		if err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

//serveBadRequest sends http.StatusBadRequest
//return true if succeeded
func serveBadRequest(w http.ResponseWriter, query string) (bool, error) {
	if query == DoBadRequest {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write(nil)
		if err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

//serveUnknownBadRequest sends http.StatusBadRequest and SearchErrorResponse
//return true if succeeded
func serveUnknownBadRequest(w http.ResponseWriter, query string) (bool, error) {
	if query == DoSearchErrorResponse {
		w.WriteHeader(http.StatusBadRequest)
		result, err := json.Marshal(SearchErrorResponse{"test error"})
		if err != nil {
			return false, err
		}
		_, err = w.Write(result)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

//serveWrongJson
func serveWrongJson(w http.ResponseWriter, query string) (bool, error) {
	if query == DoWrongJson {
		w.WriteHeader(http.StatusOK)
		result, err := json.Marshal("{ops!!!}")
		if err != nil {
			return false, err
		}
		_, err = w.Write(result)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

//return a name of field for sorting and 'ok' status with true for if the name is correct
//otherwise 'ok' status is false
func getOrderField(w http.ResponseWriter, r *http.Request) (orderField string, ok bool) {
	orderField = r.FormValue("order_field")

	ok = false

	if orderField == "" {
		orderField = allowedField[fieldNameIndex]
		ok = true
	}

	if !ok {
		for _, field := range allowedField {
			if field == orderField {
				ok = true
				break
			}
		}
	}

	return orderField, ok
}

//serveBadOrderField sends http.StatusBadRequest with SearchErrorResponse
func serveBadOrderField(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	result, err := json.Marshal(SearchErrorResponse{DoErrorBadOrderField})
	if err != nil {
		logE(err)
		return
	}
	_, err = w.Write(result)
	if err != nil {
		logE(err)
		return
	}
	return
}

//A handler to return expected results
//SearchServer emulates an external server
func SearchServer(w http.ResponseWriter, r *http.Request) {

	//Check authorization
	isAuthorized, err := serveAuthorization(w, r)
	if err != nil {
		_, err = serveInternalError(w, DoFatalError)
		if err != nil {
			logE(err)
		}
		return
	}
	if !isAuthorized {
		return
	}

	query := r.FormValue("query")

	//send StatusInternalServerError by the request
	isSuccessful, err := serveInternalError(w, query)
	if err != nil {
		logE(err)
		return
	}
	if isSuccessful {
		return
	}

	//emulate a big timeout by the request
	if serveTimeout(query) {
		return
	}

	//send StatusBadRequest by the request
	isSuccessful, err = serveBadRequest(w, query)
	if err != nil {
		logE(err)
		return
	}
	if isSuccessful {
		return
	}

	isSuccessful, err = serveUnknownBadRequest(w, query)
	if err != nil {
		logE(err)
		return
	}
	if isSuccessful {
		return
	}

	isSuccessful, err = serveWrongJson(w, query)
	if err != nil {
		logE(err)
		return
	}
	if isSuccessful {
		return
	}

	_, ok := getOrderField(w, r)
	if !ok {
		serveBadOrderField(w)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("{}"))
	if err != nil {
		_, err = serveInternalError(w, DoFatalError)
		if err != nil {
			logE(err)
		}
		return
	}
}

func TestFindUsersStatusUnauthorized(t *testing.T) {

	//goland:noinspection GoErrorStringFormat
	expect := errors.New("Bad AccessToken")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		"ExpiredToken",
		srv.URL,
	}
	_, err := sClient.FindUsers(SearchRequest{})
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersStatusBadRequest(t *testing.T) {

	expect := errors.New("cant unpack error json: unexpected end of JSON input")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      0,
		Offset:     0,
		Query:      DoBadRequest,
		OrderField: "",
		OrderBy:    0}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersErrorBadOrderField(t *testing.T) {

	expect := errors.New("OrderFeld \"WrongField\" invalid")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      0,
		Offset:     0,
		Query:      "",
		OrderField: "\"WrongField\"",
		OrderBy:    0}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersSearchErrorResponse(t *testing.T) {

	expect := errors.New("unknown bad request error: test error")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      0,
		Offset:     0,
		Query:      DoSearchErrorResponse,
		OrderField: "",
		OrderBy:    0}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersStatusInternalServerError(t *testing.T) {

	expect := errors.New("SearchServer fatal error")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      0,
		Offset:     0,
		Query:      DoFatalError,
		OrderField: "",
		OrderBy:    0}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersUnsupportedProtocolScheme(t *testing.T) {

	expect := errors.New("unknown error Get \"?limit=1&offset=0&order_by=0&order_field=&query=\": unsupported protocol scheme \"\"")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		"",
	}

	request := SearchRequest{}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersUnexpectedJson(t *testing.T) {

	expect := errors.New("cant unpack result json: json: cannot unmarshal string into Go value of type []main.User")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Query: DoWrongJson,
	}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersTimeoutTooBig(t *testing.T) {

	expect := errors.New("timeout for limit=1&offset=0&order_by=0&order_field=&query=DoBigTimeout")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      0,
		Offset:     0,
		Query:      DoBigTimeout,
		OrderField: "",
		OrderBy:    0}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersLimitTooLow(t *testing.T) {

	expect := errors.New("limit must be > 0")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      -1,
		Offset:     0,
		Query:      "",
		OrderField: "",
		OrderBy:    0}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}

func TestFindUsersOffsetTooLow(t *testing.T) {

	expect := errors.New("offset must be > 0")

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      0,
		Offset:     -1,
		Query:      "",
		OrderField: "",
		OrderBy:    0}

	_, err := sClient.FindUsers(request)
	if err == nil || expect.Error() != err.Error() {
		t.Errorf("expected an error with: %s, got: %s", expect, err)
	}

}
