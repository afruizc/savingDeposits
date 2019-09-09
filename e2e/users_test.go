package e2e

import (
	"encoding/json"
	"fmt"
	"github.com/jinzhu/gorm"
	"io/ioutil"
	"net/http"
	"savingDeposits"
	"savingDeposits/postgres"
	"savingDeposits/tst"
	"sync"
	"testing"
)

type userResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func TestCRUDUsers(t *testing.T) {
	// Arrange
	var wg sync.WaitGroup
	const addr = "localhost:8083"
	srv, clean := newServer(t)
	defer clean()

	serverUrl := fmt.Sprintf("http://%s", addr)

	wg.Add(1)
	startServer(wg, addr, srv)

	payload := []byte(`{"username":"john", "password": "secret", "role": "regular"}`)

	t.Run("CRUD user no auth, fail", func(t *testing.T) {
		// Create
		res, err := tst.MakeRequest("POST", serverUrl+"/users", "", payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusUnauthorized, "Expected 401, got %d", res.StatusCode)

		// Read
		res, err = tst.MakeRequest("GET", serverUrl+"/users", "", payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusUnauthorized,
			fmt.Sprintf("Expected 401, got %d", res.StatusCode))

		res, err = tst.MakeRequest("GET", serverUrl+"/users/1", "", payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusUnauthorized,
			fmt.Sprintf("Expected 401, got %d", res.StatusCode))

		// Update
		res, err = tst.MakeRequest("PATCH", serverUrl+"/users/1", "", payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusUnauthorized,
			fmt.Sprintf("Expected 401, got %d", res.StatusCode))

		// Delete
		res, err = tst.MakeRequest("DELETE", serverUrl+"/users/1", "", payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusUnauthorized,
			fmt.Sprintf("Expected 401, got %d", res.StatusCode))
	})

	t.Run("CRUD with regular user, should fail", func(t *testing.T) {
		user := "regular"
		// Create and get token
		_, err := createUser(user, user, user, srv.Db)
		tst.Ok(t, err)
		token, err := getUserToken(t, serverUrl, user, user)
		tst.Ok(t, err)

		// Create
		res, err := tst.MakeRequest("POST", serverUrl+"/users", token, payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusForbidden,
			fmt.Sprintf("Expected 403, got %d", res.StatusCode))

		// Read
		res, err = tst.MakeRequest("GET", serverUrl+"/users", token, payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusForbidden,
			fmt.Sprintf("Expected 403, got %d", res.StatusCode))

		res, err = tst.MakeRequest("GET", serverUrl+"/users/1", token, payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusForbidden,
			fmt.Sprintf("Expected 403, got %d", res.StatusCode))

		// Update
		res, err = tst.MakeRequest("PATCH", serverUrl+"/users/1", token, payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusForbidden,
			fmt.Sprintf("Expected 403, got %d", res.StatusCode))

		// Delete
		res, err = tst.MakeRequest("DELETE", serverUrl+"/users/1", token, payload)
		tst.Ok(t, err)
		tst.True(t, res.StatusCode == http.StatusForbidden,
			fmt.Sprintf("Expected 403, got %d", res.StatusCode))
	})

	for _, user := range []string{"manager", "admin"} {
		testName := fmt.Sprintf("CRUD user with %s, should succeed", user)
		t.Run(testName, func(t *testing.T) {
			// Create and admin
			_, err := createUser(user, user, user, srv.Db)
			tst.Ok(t, err)
			token, err := getUserToken(t, serverUrl, user, user)
			tst.Ok(t, err)

			// Create
			res, err := tst.MakeRequest("POST", serverUrl+"/users", token, payload)
			tst.Ok(t, err)
			tst.True(t, res.StatusCode == http.StatusCreated, fmt.Sprintf("Expected 201 got %d", res.StatusCode))
			rawContent, err := ioutil.ReadAll(res.Body)
			tst.Ok(t, err)

			usrRes := assertResponse(t, rawContent, "john", "regular", func(id uint) bool {
				return id != 0
			})

			// Read
			userUrl := fmt.Sprintf("%s/users/%d", serverUrl, usrRes.ID)
			res, err = tst.MakeRequest("GET", userUrl, token, payload)
			tst.Ok(t, err)
			tst.True(t, res.StatusCode == http.StatusOK, fmt.Sprintf("Expected 200 got %d", res.StatusCode))
			rawContent, err = ioutil.ReadAll(res.Body)
			tst.Ok(t, err)

			var retUser userResponse
			err = json.Unmarshal(rawContent, &retUser)
			tst.True(t, retUser.Username == usrRes.Username,
				fmt.Sprintf("Expected name %s, got %s", usrRes.Username, retUser.Username))
			tst.True(t, retUser.Role == usrRes.Role,
				fmt.Sprintf("Expected role %s, got %s", usrRes.Role, retUser.Role))
			tst.True(t, retUser.ID == usrRes.ID,
				fmt.Sprintf("Expected id %d, got %d", usrRes.ID, retUser.ID))

			// Update
			updatePayload := []byte(`{"id":100, "username": "newusr", "role": "manager"}`)
			res, err = tst.MakeRequest("PATCH", userUrl, token, updatePayload)
			tst.Ok(t, err)
			tst.True(t, res.StatusCode == http.StatusOK, fmt.Sprintf("Expected 200 got %d", res.StatusCode))
			rawContent, err = ioutil.ReadAll(res.Body)
			tst.Ok(t, err)

			var updUser userResponse
			err = json.Unmarshal(rawContent, &updUser)
			tst.True(t, updUser.Username == "john",
				fmt.Sprintf("Expected name john, got %s", updUser.Username))
			tst.True(t, updUser.Role == "manager",
				fmt.Sprintf("Expected role realtor, got %s", updUser.Role))
			tst.True(t, updUser.ID == usrRes.ID,
				fmt.Sprintf("Expected id %d, got %d", usrRes.ID, updUser.ID))

			// Delete
			res, err = tst.MakeRequest("DELETE", userUrl, token, []byte(""))
			tst.Ok(t, err)
			tst.True(t, res.StatusCode == http.StatusNoContent,
				fmt.Sprintf("Expected 204, got %d", res.StatusCode))

			res, err = tst.MakeRequest("GET", userUrl, token, []byte(""))
			tst.Ok(t, err)
			tst.True(t, res.StatusCode == http.StatusNotFound,
				fmt.Sprintf("Expected 404, got %d", res.StatusCode))
		})
	}

}

func assertResponse(t *testing.T, data []byte, user, role string, checkId func(id uint) bool) userResponse {
	var usrRes userResponse
	err := json.Unmarshal(data, &usrRes)
	tst.Ok(t, err)

	tst.True(t, usrRes.Username == user,
		fmt.Sprintf("Expected name %s, got %s", user, usrRes.Username))
	tst.True(t, usrRes.Role == role,
		fmt.Sprintf("Expected role %s, got %s", role, usrRes.Role))
	tst.True(t, checkId(usrRes.ID), "Id didn't pass check")

	return usrRes
}

func TestFetchOwnUserData(t *testing.T) {
	var wg sync.WaitGroup
	const addr = "localhost:8083"
	srv, clean := newServer(t)
	defer clean()

	serverUrl := fmt.Sprintf("http://%s", addr)

	wg.Add(1)
	startServer(wg, addr, srv)

	_, err := createUser("admin", "admin", "admin", srv.Db)
	tst.Ok(t, err)
	_, err = createUser("manager", "manager", "manager", srv.Db)
	tst.Ok(t, err)
	_, err = createUser("regular", "regular", "regular", srv.Db)
	tst.Ok(t, err)

	t.Run("Can't create user with same username", func(t *testing.T) {
		_, err := createUser("admin", "admin", "admin", srv.Db)
		tst.True(t, err != nil, "Expected error, got success")
	})

	t.Run("Read profile not logged in", func(t *testing.T) {
		// Act
		res, err := tst.MakeRequest("GET", serverUrl+"/profile", "", []byte(""))
		tst.Ok(t, err)

		// True
		tst.True(t, res.StatusCode == http.StatusUnauthorized,
			fmt.Sprintf("Expected 401, got %d", res.StatusCode))
	})

	t.Run("Read own user data regular, manager and admin", func(t *testing.T) {
		for _, user := range []string{"regular", "manager", "admin"} {
			token, err := getUserToken(t, serverUrl, user, user)
			tst.Ok(t, err)

			// Act
			res, err := tst.MakeRequest("GET", serverUrl+"/profile", token, []byte(""))
			tst.Ok(t, err)

			// True
			tst.True(t, res.StatusCode == http.StatusOK,
				fmt.Sprintf("Expected 200, got %d", res.StatusCode))

			var returnedUser savingDeposits.User
			decoder := json.NewDecoder(res.Body)
			err = decoder.Decode(&returnedUser)
			tst.Ok(t, err)

			assertUser(t, &returnedUser, user, user)
		}
	})
}

func TestCreateClient(t *testing.T) {
	var wg sync.WaitGroup
	const addr = "localhost:8083"
	srv, clean := newServer(t)
	defer clean()

	serverUrl := fmt.Sprintf("http://%s", addr)

	wg.Add(1)
	startServer(wg, addr, srv)

	_, err := createUser("admin", "admin", "admin", srv.Db)
	tst.Ok(t, err)

	t.Run("Create client, not logged in, success", func(t *testing.T) {
		// Act
		payload := []byte(`{"username": "client1", "password": "client1"}`)
		res, err := tst.MakeRequest("POST", serverUrl+"/newClient", "", payload)
		tst.Ok(t, err)

		// True
		tst.True(t, res.StatusCode == http.StatusCreated,
			fmt.Sprintf("Expected 201, got %d", res.StatusCode))

		var returnedUser savingDeposits.User
		decoder := json.NewDecoder(res.Body)
		err = decoder.Decode(&returnedUser)
		tst.Ok(t, err)

		assertUser(t, &returnedUser, "client1", "regular")
	})
}

func assertUser(t *testing.T, user *savingDeposits.User, username, role string, ) {
	tst.True(t, user.Username == username,
		fmt.Sprintf("Expected username %s, got %s", username, user.Username))

	tst.True(t, user.Role == role,
		fmt.Sprintf("Expected role %s, got %s", role, user.Role))
}

func assertNames(t *testing.T, data []byte, name, role string, testId func(id uint) bool) {

}

// Creates a user. Returns its id.
func createUser(username, pwd, role string, db *gorm.DB) (uint, error) {
	userService := postgres.NewDbUserService(db)

	result, err := userService.Create(savingDeposits.UserCreateInput{
		Username: username,
		Password: pwd,
		Role:     role,
	})
	if err != nil {
		return 0, err
	}

	return uint(result.ID), nil
}
