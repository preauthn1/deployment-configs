package vendor

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	// "sort"
	"strings"
	"time"
)

// Service is vendor api service
type Service struct {
	uri        string
	merchantid int
	key        string
}

// New get a Vendor service instance
func New(uri string, merchantid int, key string) *Service {

	svc := new(Service)
	svc.uri = uri
	svc.merchantid = merchantid
	svc.key = key

	return svc
}

// LaunchGame will send request to game/link, and return game link
func (s *Service) LaunchGame(gameid, sid, roomid int, lang, user string) (string, error) {

	u := s.uri + "/demo-site/game/link"

	v := url.Values{
		"merchantID":               []string{fmt.Sprintf("%d", s.merchantid)},
		"merchantSecretKey":        []string{s.key},
		"playerName":               []string{user},
		"lang":                     []string{lang},
		"gameID":                   []string{fmt.Sprintf("%d", gameid)},
		"secretParameter_ServerID": []string{fmt.Sprintf("%d", sid)},
		// "roomLevelFlagID":   []string{fmt.Sprintf("%d", roomid)},
	}

	u += "?" + v.Encode()

	log.Printf("Launch Game: %s\n", u)

	resp, err := http.Get(u)
	if err != nil {
		return "", fmt.Errorf("Get Launch Game Error, %v", err)
	}
	defer resp.Body.Close()

	type res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Link string `json:"link"`
		} `json:"data"`
	}

	data := new(res)

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(data); err != nil {
		return "", fmt.Errorf("Parse Launch Game Error, %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Get Launch Game Error, Status %d, %s", resp.StatusCode, data.Message)
	}

	return data.Data.Link, nil
}

// SingleWalletLaunchGame will send request to game/link, and return game link
func (s *Service) SingleWalletLaunchGame(gameid, sid, roomid int, lang, user string) (string, error) {

	u := s.uri + "/demo-site/game/launch-link"

	v := url.Values{
		"merchantID":               []string{fmt.Sprintf("%d", s.merchantid)},
		"merchantSecretKey":        []string{s.key},
		"PlayerUniqueID":           []string{user},
		"lang":                     []string{lang},
		"gameID":                   []string{fmt.Sprintf("%d", gameid)},
		"SessionToken":             []string{fmt.Sprintf("%d", time.Now().UnixNano())},
		"secretParameter_ServerID": []string{fmt.Sprintf("%d", sid)},
		// "roomLevelFlagID":   []string{fmt.Sprintf("%d", roomid)},
	}

	u += "?" + v.Encode()

	log.Printf("Launch Game: %s\n", u)

	resp, err := http.Get(u)
	if err != nil {
		return "", fmt.Errorf("Get Launch Game Error, %v", err)
	}
	defer resp.Body.Close()

	type res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Link string `json:"link"`
		} `json:"data"`
	}

	data := new(res)

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(data); err != nil {
		return "", fmt.Errorf("Parse Launch Game Error, %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Get Launch Game Error, Status %d, %s", resp.StatusCode, data.Message)
	}

	return data.Data.Link, nil
}

// GameList Get game list for vendor
func (s *Service) GameList(lang string) ([]*ServerGroup, error) {

	if len(lang) == 0 {
		lang = "en-us"
	}

	u := fmt.Sprintf("%s/demo-site/merchant/%d/servers", s.uri, s.merchantid)

	log.Printf("List Game: %s\n", u)

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("Get GameList Error, Create Request Error, %v", err)
	}

	req.Header.Set("locale", strings.ToLower(lang))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Get GameList Error, %v", err)
	}

	type res struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    []*ServerGroup `json:"data"`
	}

	data := new(res)

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(data); err != nil {
		return nil, fmt.Errorf("Parse GameList Error, %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Get GameList Error, Status %d, %s", resp.StatusCode, data.Message)
	}

	// for _, sg := range data.Data {
	// 	sort.Slice(sg.Games, func(i, j int) bool {
	// 		return sg.Games[i].ID > sg.Games[j].ID
	// 	})
	// // }

	// fmt.Println("Hello World!")

	// for _, sg := range data.Data {
	// 	for _, game := range sg.Games {		
	// 		fmt.Printf("%s", game.ID)
	// 	}
	// }

	return data.Data, nil

}
