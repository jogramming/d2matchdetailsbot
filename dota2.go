package main

import (
	"encoding/json"
	simplejson "github.com/bitly/go-simplejson"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
)

const AccountIdAdd = "76561197960265728"

type AbilityUpgrade struct {
	Ability int
	Time    int
	Level   int
}

type Player struct {
	Account_id    int
	Persona_name  string //Not filled out by the web api
	Item_string   string //not filled out by the web api
	Player_slot   int
	Hero_id       int
	Item_0        int
	Item_1        int
	Item_2        int
	Item_3        int
	Item_4        int
	Item_5        int
	Kills         int
	Deaths        int
	Assists       int
	Leaver_status int
	Gold          int
	Last_hits     int
	Denies        int
	Gold_per_min  int
	Xp_per_min    int
	Gold_spent    int
	Hero_damage   int
	Tower_damage  int
	Hero_healing  int
	Level         int

	Ability_upgrades []AbilityUpgrade
}

type Pick_Ban struct {
	Is_pick bool
	Hero_id int
	Team    int
	Order   int
}

type MatchDetailsResult struct {
	Players    []Player
	Picks_bans []Pick_Ban

	Radiant_win             bool
	Duration                int
	Start_time              int
	Match_id                int
	Match_seq_num           int
	Tower_staus_radiant     int
	Tower_status_dire       int
	Barracks_status_radiant int
	Barracks_status_dire    int
	Cluster                 int
	First_blood_time        int
	Lobby_type              int
	Human_players           int
	Leagueid                int
	Positive_votes          int
	Negative_votes          int
	Game_mode               int

	Radiant_name string
	Dire_name    string

	Game_mode_str string //Not filled out by api
}

var GameModes = []string{
	"None",
	"All Pick",
	"Captain's Mode",
	"Random Draft",
	"Single Draft",
	"All Random",
	"Unknown",
	"Diretide",
	"Reverse Captain's Mode",
	"The Greeviling",
	"Tutorial",
	"Mid Only",
	"Least Played",
	"Limited Heroes",
	"Compendium",
	"Custom",
	"Captain's Draft",
}

type MatchDetails struct {
	Result MatchDetailsResult
}

func GetMatchDetails(key, id string) (*MatchDetailsResult, error) {
	resp, err := http.Get("https://api.steampowered.com/IDOTA2Match_570/GetMatchDetails/V001/?match_id=" + id + "&key=" + key)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var md MatchDetails
	err = json.Unmarshal(body, &md)
	if err != nil {
		return nil, err
	}

	return &md.Result, nil
}

type GetHeroesResponse struct {
	Result GetHeroesResult
}

type GetHeroesResult struct {
	Heroes []GetHeroHero
	Count  int
}

type GetHeroHero struct {
	Name           string
	Id             int
	Localized_name string
}

type HeroListing struct {
	Listing []string
}

func (h *HeroListing) Get(i int) string {
	return h.Listing[i]
}

func CreateHeroListing(key string) (*HeroListing, error) {
	resp, err := http.Get("http://api.steampowered.com/IEconDOTA2_570/GetHeroes/v0001/?language=en&key=" + key)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rs GetHeroesResponse
	err = json.Unmarshal(body, &rs)
	if err != nil {
		return nil, err
	}
	highest := 0
	for _, v := range rs.Result.Heroes {
		if v.Id > highest {
			highest = v.Id
		}
	}
	listing := make([]string, highest+1)
	for _, v := range rs.Result.Heroes {
		listing[v.Id] = v.Localized_name
	}
	log.Printf(ENDC+"Loaded %d Heroes\n", rs.Result.Count)
	return &HeroListing{listing}, nil
}

type ItemListing struct {
	Listing []string
}

func (l *ItemListing) Get(i int) string {
	return l.Listing[i]
}

func CreateItemListing() (*ItemListing, error) {
	resp, err := http.Get("http://www.dota2.com/jsfeed/itemdata")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	js, err := simplejson.NewJson(body)

	if err != nil {
		return nil, err
	}

	idata := js.Get("itemdata")
	mapped, _ := idata.Map()
	highest := 0.0

	//first we get the number of items
	for _, v := range mapped {
		det := v.(map[string]interface{})
		id := det["id"].(float64)
		if id > highest {
			highest = id
		}
	}

	//Then we actually add them
	listing := make([]string, int(highest)+1)
	for _, v := range mapped {
		det := v.(map[string]interface{})
		id := det["id"].(float64)
		name := det["dname"].(string)
		listing[int(id)] = name
	}

	log.Printf(ENDC+"Loaded %d Items\n", int(highest))
	return &ItemListing{listing}, nil
}

type PlayerSum struct {
	Response PlayersSumResponse
}

type PlayersSumResponse struct {
	Players []PlayerSumPlayer
}

type PlayerSumPlayer struct {
	Steamid     string
	Personaname string
}

func GetNamesFromAccuntIds(key string, players []Player) ([]PlayerSumPlayer, error) {
	url := "http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=" + key + "&steamids="
	for _, v := range players {
		id := v.Account_id
		realid := convertTo64Bit(int64(id))

		url += realid + ","
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ps PlayerSum
	err = json.Unmarshal(body, &ps)
	if err != nil {
		return nil, err
	}
	return ps.Response.Players, nil
}

func convertTo64Bit(id int64) string {
	bid := big.NewInt(id)

	modifier := big.NewInt(0)
	modifier, _ = modifier.SetString(AccountIdAdd, 10)

	realId := bid.Add(bid, modifier)
	return realId.String()
}
