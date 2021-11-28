package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
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

//A struct which describes testing data in xml
type Root struct {
	XMLName xml.Name  `xml:"root"`
	Rows    []RowItem `xml:"row"`
}

//An implementation of fmt.Stringer() interface for Root structure
func (root Root) String() string {
	return "root with " + strconv.Itoa(len(root.Rows)) + " rows"
}

//An implementation of fmt.Stringer() interface for User structure
func (usr User) String() string {
	return "user with {id:" + strconv.Itoa(usr.Id) + ", name:" + usr.Name + ", age:" + strconv.Itoa(usr.Age) + "}"
}

//A struct which describes testing data in xml
type RowItem struct {
	XMLName       xml.Name `xml:"row"`
	ID            string   `xml:"id"`
	Guid          string   `xml:"guid"`
	IsActive      string   `xml:"isActive"`
	Balance       string   `xml:"balance"`
	Picture       string   `xml:"picture"`
	Age           string   `xml:"age"`
	EyeColor      string   `xml:"eyeColor"`
	FirstName     string   `xml:"first_name"`
	LastName      string   `xml:"last_name"`
	Gender        string   `xml:"gender"`
	Company       string   `xml:"company"`
	Email         string   `xml:"email"`
	Phone         string   `xml:"phone"`
	Address       string   `xml:"address"`
	About         string   `xml:"about"`
	Registered    string   `xml:"registered"`
	FavoriteFruit string   `xml:"favoriteFruit"`
}

//get a new User from RowItem
func (row RowItem) newUser() *User {

	id, err := strconv.Atoi(row.ID)
	if err != nil {
		panic(err)
	}

	age, err := strconv.Atoi(row.Age)
	if err != nil {
		panic(err)
	}

	user := &User{
		Id:     id,
		Name:   row.FirstName + row.LastName,
		Age:    age,
		Gender: row.Gender,
		About:  row.About,
	}
	return user
}

//parsing dataset.xml
func parseTestData() (*Root, error) {
	xmlFile, err := os.Open("dataset.xml")
	if err != nil {
		return nil, err
	}
	defer func() {
		err := xmlFile.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()
	byteValue, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return nil, err
	}
	rss := Root{}
	err = xml.Unmarshal(byteValue, &rss)
	if err != nil {
		return nil, err
	}
	return &rss, err
}

//getUsersFromRows converts a slice of RowItem to a slice of User
func getUsersFromRows(rows []RowItem) (users []User) {
	users = make([]User, 0, len(rows))
	for _, row := range rows {
		usr := row.newUser()
		if usr != nil {
			users = append(users, *usr)
		}
	}
	return
}

func sortByFieldName(fieldName string, order int, users []User) {

	idComparatorA := func(i, j int) bool {
		return users[i].Id < users[j].Id
	}
	idComparatorD := func(i, j int) bool {
		return users[i].Id > users[j].Id
	}

	ageComparatorA := func(i, j int) bool {
		return users[i].Age < users[j].Age
	}
	ageComparatorD := func(i, j int) bool {
		return users[i].Age > users[j].Age
	}

	nameComparatorA := func(i, j int) bool {
		return users[i].Name < users[j].Name
	}
	nameComparatorD := func(i, j int) bool {
		return users[i].Name > users[j].Name
	}

	var useForId func(int, int) bool
	var useForAge func(int, int) bool
	var useForName func(int, int) bool

	if order == OrderByAsc {
		useForId = idComparatorA
		useForAge = ageComparatorA
		useForName = nameComparatorA
	} else if order == OrderByDesc {
		useForId = idComparatorD
		useForAge = ageComparatorD
		useForName = nameComparatorD
	} else {
		return
	}

	switch fieldName {

	case allowedField[fieldIdIndex]:
		sort.Slice(users, useForId)

	case allowedField[fieldAgeIndex]:
		sort.Slice(users, useForAge)

	default: //fieldNameIndex
		sort.Slice(users, useForName)

	}

}

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
func serveBadRequestForTest(w http.ResponseWriter, query string) (bool, error) {
	if query == DoBadRequest {
		err := serveBadRequest(w, "")
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
		err := serveBadRequest(w, "test error")
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
func getOrderField(r *http.Request) (orderField string, ok bool) {
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

//return an offset from FormValue
//or an error
func getOffset(r *http.Request) (offset int, err error) {
	offsetValue := r.FormValue("offset")
	offset, err = strconv.Atoi(offsetValue)
	return
}

//return a limit from FormValue
//or an error
func getLimit(r *http.Request) (limit int, err error) {
	limitValue := r.FormValue("limit")
	limit, err = strconv.Atoi(limitValue)
	return
}

//serveBadOrderField sends http.StatusBadRequest with SearchErrorResponse
func serveBadOrderField(w http.ResponseWriter) {
	err := serveBadRequest(w, DoErrorBadOrderField)
	if err != nil {
		logE(err)
		return
	}
	return
}

//send http.StatusBadRequest with 'errorDescription' in SearchErrorResponse
func serveBadRequest(w http.ResponseWriter, errorDescription string) error {
	w.WriteHeader(http.StatusBadRequest)
	var bytes []byte
	var err error
	if errorDescription != "" {
		bytes, err = json.Marshal(SearchErrorResponse{errorDescription})
		if err != nil {
			return err
		}
	}
	_, err = w.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}

//applyFilterUsers gets a slice of User
//which contain 'search' value in User.About or User.Name
func applyFilterUsers(usersIncome []User, search string) (found []User) {
	if search == "" {
		return usersIncome[:]
	}
	found = make([]User, 0, len(usersIncome))
	for _, user := range usersIncome {
		if strings.Contains(user.Name, search) || strings.Contains(user.About, search) {
			found = append(found, user)
		}
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
	isSuccessful, err = serveBadRequestForTest(w, query)
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

	fieldOrder, ok := getOrderField(r)
	if !ok {
		serveBadOrderField(w)
		return
	}

	offset, err := getOffset(r)
	if err != nil {
		err = serveBadRequest(w, "wrong 'offset' parameter")
		if err != nil {
			logE(err)
		}
		return
	}

	limit, err := getLimit(r)
	if err != nil {
		err = serveBadRequest(w, "wrong 'limit' parameter")
		if err != nil {
			logE(err)
		}
		return
	}

	orderBy := OrderByAsIs

	switch r.FormValue("order_by") {
	case "-1":
		orderBy = OrderByAsc
	case "1":
		orderBy = OrderByDesc
	}

	root, err := parseTestData()
	if err != nil {
		logE(err)
		_, err = serveInternalError(w, DoFatalError)
		if err != nil {
			logE(err)
		}
		return
	}
	if root == nil {
		logE(errors.New("no test data"))
		_, err = serveInternalError(w, DoFatalError)
		if err != nil {
			logE(err)
		}
		return
	}

	users := getUsersFromRows(root.Rows)
	if len(users) == 0 {
		logE(errors.New("no users"))
		_, err = serveInternalError(w, DoFatalError)
		if err != nil {
			logE(err)
		}
		return
	}

	if orderBy != OrderByAsIs {
		sortByFieldName(fieldOrder, orderBy, users)
	}
	if limit == 0 {
		limit = len(users)
	}
	var result []byte

	//From task:
	//If `query` is empty, then we do only sorting, i.e. return all records
	if query != "" {
		users = applyFilterUsers(users, query)
	}
	if limit > len(users) {
		limit = len(users)
	}

	//a logic of right offset is skipped
	//so this statement just preventing error
	if limit+offset > len(users) {
		_ = serveBadRequest(w, "wrong 'offset' parameter")
		return
	}

	limitedResult := users[offset:(limit + offset)]
	result, err = json.Marshal(limitedResult)

	if err != nil {
		logE(err)
		_, err = serveInternalError(w, DoFatalError)
		if err != nil {
			logE(err)
		}
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(result)
	if err != nil {
		logE(err)
		_, err = serveInternalError(w, DoFatalError)
		if err != nil {
			logE(err)
		}
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

func TestFindUsersNoQuery(t *testing.T) {

	expectUsers := 10

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "",
		OrderField: "",
		OrderBy:    OrderByAsc}

	response, err := sClient.FindUsers(request)
	if err != nil {
		t.Errorf("expected no error, got: %s", err)
		return
	}
	if response == nil {
		t.Errorf("expected response, got: nil")
		return
	}
	if len(response.Users) != expectUsers {
		t.Errorf("expected users: %d, got: %d", expectUsers, len(response.Users))
	}

}

func TestFindUsersQueryByName(t *testing.T) {

	expectUsers := 1

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "ann",
		OrderField: "",
		OrderBy:    OrderByAsc}

	response, err := sClient.FindUsers(request)
	if err != nil {
		t.Errorf("expected no error, got: %s", err)
		return
	}
	if response == nil {
		t.Errorf("expected response, got: nil")
		return
	}
	if len(response.Users) != expectUsers {
		t.Errorf("expected users: %d, got: %d", expectUsers, len(response.Users))
	}

}

func TestFindUsersQueryByAbout(t *testing.T) {

	expectUsers := 1

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Nulla cillum enim",
		OrderField: "",
		OrderBy:    OrderByAsc}

	response, err := sClient.FindUsers(request)
	if err != nil {
		t.Errorf("expected no error, got: %s", err)
		return
	}
	if response == nil {
		t.Errorf("expected response, got: nil")
		return
	}
	if len(response.Users) != expectUsers {
		t.Errorf("expected users: %d, got: %d", expectUsers, len(response.Users))
	}

}

func TestFindUsersBigLimit(t *testing.T) {

	expectUsers := 25

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      30,
		Offset:     0,
		Query:      "",
		OrderField: "",
		OrderBy:    OrderByAsIs}

	response, err := sClient.FindUsers(request)
	if err != nil {
		t.Errorf("expected no error, got: %s", err)
		return
	}
	if response == nil {
		t.Errorf("expected response, got: nil")
		return
	}
	if len(response.Users) != expectUsers {
		t.Errorf("expected users: %d, got: %d", expectUsers, len(response.Users))
	}

}

func TestFindUsersSortAgeByAsc(t *testing.T) {

	expectUsers := []User{{Age: 29}, {Age: 30}, {Age: 31}}

	expectUsersNumber := len(expectUsers)

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      3,
		Offset:     0,
		Query:      "nn",
		OrderField: "Age",
		OrderBy:    OrderByAsc}

	response, err := sClient.FindUsers(request)
	if err != nil {
		t.Errorf("expected no error, got: %s", err)
		return
	}
	if response == nil {
		t.Errorf("expected response, got: nil")
		return
	}
	if len(response.Users) != expectUsersNumber {
		t.Errorf("expected users: %d, got: %d", expectUsersNumber, len(response.Users))
	}
	for i, expectUser := range expectUsers {
		if expectUser.Age != response.Users[i].Age {
			t.Errorf("expected user's age: %d, got: %d", expectUser.Age, response.Users[i].Age)
		}
	}

}

func TestFindUsersSortAgeByDesc(t *testing.T) {

	expectUsers := []User{{Age: 39}, {Age: 35}, {Age: 34}}

	expectUsersNumber := len(expectUsers)

	srv := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer srv.Close()

	sClient := &SearchClient{
		OkToken,
		srv.URL,
	}

	request := SearchRequest{
		Limit:      3,
		Offset:     0,
		Query:      "nn",
		OrderField: "Age",
		OrderBy:    OrderByDesc}

	response, err := sClient.FindUsers(request)
	if err != nil {
		t.Errorf("expected no error, got: %s", err)
		return
	}
	if response == nil {
		t.Errorf("expected response, got: nil")
		return
	}
	if len(response.Users) != expectUsersNumber {
		t.Errorf("expected users: %d, got: %d", expectUsersNumber, len(response.Users))
	}
	for i, expectUser := range expectUsers {
		if expectUser.Age != response.Users[i].Age {
			t.Errorf("expected user's age: %d, got: %d", expectUser.Age, response.Users[i].Age)
		}
	}

}
