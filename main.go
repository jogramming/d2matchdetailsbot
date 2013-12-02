package main

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"github.com/jonas747/reddit"
	"io/ioutil"
	"log"
	"strings"
	"text/template"
	"time"
)

const (
	HEADER  = "\033[95m"
	OKBLUE  = "\033[94m"
	OKGREEN = "\033[92m"
	WARNING = "\033[93m"
	FAIL    = "\033[91m"
	ENDC    = "\033[0m"
)

var (
	config          Config
	heroListing     *HeroListing
	itemListing     *ItemListing
	rAccount        *reddit.Account
	commentTemplate *template.Template

	checkdeComments list.List
)

func main() {
	log.SetPrefix(OKBLUE)
	log.Println(OKGREEN + "Starting..." + ENDC)
	configa, err := LoadConfig("config.json")
	config = *configa
	if err != nil {
		panic(err)
	}

	log.Println(OKGREEN + "Getting hero listing" + ENDC)
	heroListing, err = CreateHeroListing(config.D2Key)
	if err != nil {
		panic(err)
	}

	log.Println(OKGREEN + "Getting item listing" + ENDC)
	itemListing, err = CreateItemListing()
	if err != nil {
		panic(err)
	}

	log.Println(OKGREEN + "Loading and parsing comment template")
	file, err := ioutil.ReadFile("template.txt")
	if err != nil {
		panic(err)
	}

	funcMap := template.FuncMap{
		// The name "title" is what the function will be called in the template text.
		"equals":         Equals,
		"heroname":       GetHeroName,
		"FormatDuration": FormatDuration,
	}

	commentTemplate, err = template.New("comment").Funcs(funcMap).Parse(string(file))
	if err != nil {
		panic(err)
	}

	log.Println(OKGREEN + "Sucess! Starting the comment stream..." + ENDC)
	// If any issues happens we log in again and start the stream again
	for {
		after := time.After(time.Duration(10) * time.Second)

		StartStream()

		<-after
	}
}

func PostMatchDetails(parent string, matches []string, account reddit.Account) error {
	matchDetails := make([]*MatchDetailsResult, 0, 0)
	for _, match := range matches {
		details, err := GetMatchDetails(config.D2Key, match)
		if err != nil {
			return err
		}

		playersums, err := GetNamesFromAccuntIds(config.D2Key, details.Players)
		if err != nil {
			return err
		}
		if len(details.Players) < 10 {
			log.Println(FAIL + "details.Players is less than 10 in matchid " + match + ", Skipping" + ENDC)
			continue
		}
		for i := 0; i < 10; i++ {
			id := details.Players[i].Account_id
			realid := convertTo64Bit(int64(id))

			//Find the name
			name := "Private Profile"
			for _, psum := range playersums {
				if psum.Steamid == realid {
					name = "[" + psum.Personaname + "](http://steamcommunity.com/profiles/" + realid + ")"
					break
				}
			}

			details.Players[i].Persona_name = strings.Replace(name, "|", " ", -1)

			if details.Players[i].Item_0 != 0 {
				details.Players[i].Item_string += itemListing.Get(details.Players[i].Item_0) + ", "
			}
			if details.Players[i].Item_1 != 0 {
				details.Players[i].Item_string += itemListing.Get(details.Players[i].Item_1) + ", "
			}
			if details.Players[i].Item_2 != 0 {
				details.Players[i].Item_string += itemListing.Get(details.Players[i].Item_2) + ", "
			}
			if details.Players[i].Item_3 != 0 {
				details.Players[i].Item_string += itemListing.Get(details.Players[i].Item_3) + ", "
			}
			if details.Players[i].Item_4 != 0 {
				details.Players[i].Item_string += itemListing.Get(details.Players[i].Item_4) + ", "
			}
			if details.Players[i].Item_5 != 0 {
				details.Players[i].Item_string += itemListing.Get(details.Players[i].Item_5) + ", "
			}
			if len(details.Players[i].Item_string) > 2 {
				details.Players[i].Item_string = details.Players[i].Item_string[:len(details.Players[i].Item_string)-2]
			}
		}
		details.Game_mode_str = GameModes[details.Game_mode]
		matchDetails = append(matchDetails, details)
	}
	if len(matchDetails) == 0 {
		return errors.New("Nothing to post")
	}
	writer := new(bytes.Buffer)
	commentTemplate.Execute(writer, matchDetails)
	account.ReplyToThing(parent, writer.String())
	return nil
}

func Equals(a, b int) bool {
	return a == b
}
func GetHeroName(id int) string {
	return heroListing.Get(id)
}

func FormatDuration(duration int) string {
	mins := duration / 60
	sec := duration % 60
	return fmt.Sprintf("%d:%d", mins, sec)
}

func StartStream() {
	rAccount, err := reddit.Login(config.RUser, config.RPass, "Dota 2 matchdetails bot. /u/jonas747")
	if err != nil {
		log.Println("Failed logging in, trying again in 10 seconds")
		return
	}
	cStream := &reddit.CommentStream{
		Update:        make(chan reddit.Comment),
		Stop:          make(chan bool),
		Errors:        make(chan error),
		FetchInterval: time.Duration(10) * time.Second,
		Subreddit:     config.RSub,
		RAccount:      rAccount,
	}
	go cStream.Run()
	ticker := time.NewTicker(time.Duration(10) * time.Minute)
	numFound := 0
	numProcessed := 0
	for {
		select {
		case comment := <-cStream.Update:
			if checkIfCommentChecked(comment.FullName) || comment.Author == rAccount.Username || comment.Author == "jonas747_bot" {

				continue
			}
			addCheckedComment(comment.FullName)
			ids := CheckContainsMatchId(comment.Body)
			if len(ids) > 0 {
				for _, id := range ids {
					log.Println(ENDC + "Found match id: " + id)
					log.Println(ENDC + "In: " + comment.Body)
					numFound++
				}
				PostMatchDetails(comment.FullName, ids, *rAccount)
			}
			numProcessed++
		case err := <-cStream.Errors:
			if err == reddit.ERRCOMMENTSVOID {
				log.Println(FAIL + "Comment void, restarting stream" + ENDC)
				ticker.Stop()
				return
			}
			log.Println(FAIL + err.Error() + ENDC)
		case <-ticker.C:
			log.Printf(ENDC+"Processed %d comments, found %d matches the last 10 minutes", numProcessed, numFound)
			numFound = 0
			numProcessed = 0
		}
	}

}

func cleanCheckedComments() {
	for checkdeComments.Len() > 1000 {
		elem := checkdeComments.Front()
		checkdeComments.Remove(elem)
	}
}

func addCheckedComment(id string) {
	checkdeComments.PushBack(id)
	if checkdeComments.Len() > 1000 {
		cleanCheckedComments()
	}
}

func checkIfCommentChecked(id string) bool {
	for next := checkdeComments.Front(); next != nil; next = next.Next() {
		if next.Value == id {
			return true
		}
	}
	return false
}

func CheckContainsMatchId(comment string) []string {
	lower := strings.ToLower(comment)
	wordmatches := []string{"matchid", "match-id", "match id", "dotabuff.com/matches/"}
	results := make([]string, 0, 0)
	for {
		if len(results) > 5 {
			break
		}
		found, word := stringContainOneOf(lower, wordmatches)
		if found != -1 {
			numbers := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
			recordingNumber := false
			numString := ""
			//check for numbers after one of the words
			for i := found; i < len(lower); i++ {
				foundNumber, _ := stringContainOneOf(string(lower[i]), numbers)
				if recordingNumber {
					if foundNumber != -1 {
						numString += string(lower[i])
					} else {
						break
					}
				} else {
					if foundNumber != -1 {
						numString += string(lower[i])
						recordingNumber = true
					} else if i > found+len(word)+5 {
						break
					}
				}

			}
			if numString != "" {
				results = append(results, numString)
				lower = lower[found+len(word):]
				continue
			}
			//if that failed, check before the word
			recordingNumber = false
			for i := found; i >= 0; i-- {
				foundNumber, _ := stringContainOneOf(string(lower[i]), numbers)
				if recordingNumber {
					if foundNumber != -1 {
						numString = string(lower[i]) + numString
					} else {
						break
					}
				} else {
					if foundNumber != -1 {
						numString = string(lower[i]) + numString
						recordingNumber = true
					} else if i-found < -20 {
						break
					}
				}

			}
			if numString == "" {
				break
			} else {
				lower = lower[found+len(word):]
				results = append(results, numString)
			}
		} else {
			break
		}
	}
	return results
}

func stringContainOneOf(str string, contains []string) (int, string) {
	lowestIndex := -1
	lowestWord := ""
	for _, v := range contains {
		if strings.Contains(str, v) {
			if strings.Index(str, v) < lowestIndex || lowestIndex == -1 {
				lowestIndex = strings.Index(str, v)
				lowestWord = v
			}
		}
	}
	return lowestIndex, lowestWord
}
