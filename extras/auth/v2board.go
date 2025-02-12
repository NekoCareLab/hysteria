package auth

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/apernet/hysteria/core/v2/server"
)

var _ server.Authenticator = &V2boardApiProvider{}

type V2boardApiProvider struct {
	Client *http.Client
	URL    string
}

// User List
var (
	usersMap map[string]User
	lock     sync.Mutex
)

type User struct {
	ID         int     `json:"id"`
	UUID       string  `json:"uuid"`
	SpeedLimit *uint32 `json:"speed_limit"`
}
type ResponseData struct {
	Users []User `json:"users"`
}

func getUserList(url string) ([]User, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responseData ResponseData
	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		return nil, err
	}

	return responseData.Users, nil
}

func UpdateUsers(url string, interval time.Duration, trafficlogger server.TrafficLogger) {
	fmt.Println("Start pulling user list from the remote per", interval)

	// Upgrade upfront
	userList, err := getUserList(url)
	if err != nil {
		fmt.Println("Error:", err)
		return // Throw if it's empty
	}
	processUserList(userList, trafficlogger)

	// Timer
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		userList, err := getUserList(url)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}
		processUserList(userList, trafficlogger)
	}
}

func processUserList(userList []User, trafficlogger server.TrafficLogger) {
	lock.Lock()
	defer lock.Unlock()

	newUsersMap := make(map[string]User)
	for _, user := range userList {
		newUsersMap[user.UUID] = user
	}
	if trafficlogger != nil {
		for uuid := range usersMap {
			if _, exists := newUsersMap[uuid]; !exists {
				fmt.Println(usersMap[uuid].ID)
				trafficlogger.NewKick(strconv.Itoa(usersMap[uuid].ID))
			}
		}
	}

	usersMap = newUsersMap
}

func (v *V2boardApiProvider) Authenticate(addr net.Addr, auth string, tx uint64) (ok bool, id string) {

	lock.Lock()
	defer lock.Unlock()

	if user, exists := usersMap[auth]; exists {
		return true, strconv.Itoa(user.ID)
	}
	return false, ""
}