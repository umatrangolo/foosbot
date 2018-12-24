package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

// Checks if the command comes from Slack
func exSignedReqBody(r *http.Request, secret []byte) (signedReqBody *[]byte, err error) {
	// ref: http://localhost:8088/pkg/crypto/hmac/
	var checkMAC = func(message, messageMAC, key []byte) bool {
		mac := hmac.New(sha256.New, key)
		mac.Write(message)
		expectedMAC := mac.Sum(nil)
		return string(messageMAC) == hex.EncodeToString(expectedMAC)
	}

	ts, messageMAC := r.Header["X-Slack-Request-Timestamp"][0], strings.Split(r.Header["X-Slack-Signature"][0], "=")[1]
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	message := fmt.Sprintf("%s:%s:%s", "v0", ts, b)

	if !checkMAC([]byte(message), []byte(messageMAC), secret) {
		return nil, fmt.Errorf("Failed HMAC check")
	}

	return &b, nil
}

type State = uint8
const (
	NoGame State = iota
	WaitingForPlayers
)

type Command = string
const (
	Explain  Command = "/explain"
	GiveUp Command = "/giveup"
	New   Command = "/new"
	Play  Command = "/play"
	Reset Command = "/reset"
	Current Command = "/current"
)

type Status struct {
	State State
	Players map[string]int
}
var current = Status{
	State: NoGame,
	Players: map[string]int{},
}

func (s Status) getPlayers() []string {
	players := make([]string, 0, len(s.Players))
	for uid, _ := range s.Players {
		players = append(players, fmt.Sprintf("<@%s>", uid))
	}
	return players
}

// Trying to be fair ...
func mkMissingPlayers(n int) []string {
	missing := make([]string, 0, n)
	for n > 0 {
		r := rand.Int() % 2
		if r == 0 {
			missing = append(missing, ":man:")
		} else {
			missing = append(missing, ":woman:")
		}
		n--
	}
	return missing
}

const ExplainMessage = "Available commands:\n\t*/new*\t\tStarts a new game\n\t*/play*\t\tJoins current game\n\t*/giveup*\tAbandon current game\n\t*/reset*\t\tHard reset\n\t*/current*\t\tShow status\n"

type Attachment struct {
	Text string `json:"text"`
}
type Response struct {
	Type string `json:"response_type"`
	Text string `json:"text"`
	Attachments []Attachment `json:"attachments"`
}

// Each time a player gets addes to `current` we assign him/her a
// random score that then we use shiffle players to build the teams.
type challenger struct{
	uid string
	score int
}

type byScore []challenger
func (x byScore) Len() int { return len(x) }
func (x byScore) Less(i, j int) bool { return x[i].score < x[j].score }
func (x byScore) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func mkTeams(players map[string]int) (left [2]string, right [2]string) {
	challengers := make([]challenger, 4)
	i := 0
	for p, s := range players {
		challengers[i] = challenger{p, s}
		i++
	}

	sort.Sort(byScore(challengers))

	left[0] = challengers[0].uid
	left[1] = challengers[1].uid
	right[0] = challengers[0].uid
	right[1] = challengers[1].uid

	return left, right
}

func parseCommand(r *http.Request) (cmd, user, userId string, err error) {
	sreq, err := exSignedReqBody(r, secret)
	if err != nil {
		return "", "", "", err
	}
	values, err := url.ParseQuery(string(*sreq))
	if err != nil {
		return "", "", "", err
	}
	log.Printf("%s", sreq)
	cmd = values["command"][0]
	user = values["user_name"][0]
	userId = values["user_id"][0]
	return cmd, user, userId, nil
}

func mkSlackResp(w http.ResponseWriter, text string, texts []string) error {
	w.Header().Set("Content-Type", "application/json")

	attachments := []Attachment{}
	for _, t := range texts {
		attachments = append(attachments, Attachment{Text: t})
	}

	resp := Response{
		Type: "in_channel",
		Text: text,
		Attachments: attachments,
	}

	jzon, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	//log.Printf("json: %s", string(jzon))
	fmt.Fprintf(w, string(jzon))
	return nil
}

var secret []byte
func init() {
	s, ok := os.LookupEnv("SECRET")
	if !ok {
		log.Fatalf("You must setup the SECRET env val")
	}
	secret = []byte(s)
}

func main() {
	log.Println("Starting [foosbot] slack app")

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s [%v] (%v) : %+v", r.Method, r.URL, r.RemoteAddr, r)
		_, err := exSignedReqBody(r, secret)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		fmt.Fprintf(w, "pong")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s [%v] (%v) : %+v", r.Method, r.URL, r.RemoteAddr, r)
		command, user, userId, err := parseCommand(r)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		switch current.State {
		case NoGame:
			if command == Play || command == GiveUp || command == Current || command == Reset {
				log.Printf("No open game")
				msg := fmt.Sprintf("<@%s> There is no open game", userId)
				mkSlackResp(w, msg, []string{"Use the */new* command to start a new one"})
			} else if command == New {
				log.Printf("User [%s] created a new game.", user)
				current.State = WaitingForPlayers
				current.Players = map[string]int{}
				current.Players[userId] = rand.Int()
				log.Printf("[WaitingForPlayers] players: %v", current.Players)
				w.Header().Set("Content-Type", "application/json")
				msg := fmt.Sprintf("<!here> User <@%s> just started a new game", userId)
				mkSlackResp(w, msg, []string{"Use */play* to join"})
			} else if command == Explain {
				mkSlackResp(w, ExplainMessage, []string{})
			} else {
				log.Printf("[NoGame] Unrecognized command: [%s]", command)
				mkSlackResp(w, "Unrecognized command", []string{"Use */explain* for a list of all available commands"})
			}
		case WaitingForPlayers:
			_, ok := current.Players[userId]
			if command == Play {
				if !ok {
					log.Printf("Adding [%s] to the current game", user)
					current.Players[userId] = rand.Int()
					log.Printf("[WaitingForPlayers] players: %v", current.Players)
					if len(current.Players) == 4 {
						firstTeam, secondTeam := mkTeams(current.Players)
						msg := fmt.Sprintf("[<@%s> - <@%s>] vs. [<@%s> - <@%s>]", firstTeam[0], firstTeam[1], secondTeam[0], secondTeam[1])
						mkSlackResp(w, msg, []string{"<!here> :bell::soccer: *Game is on!* :bell::soccer:"})
						current = Status{
							State: NoGame,
							Players: map[string]int{},
						}
					} else {
						msg := fmt.Sprintf("<@%s> you have been added to the current game", userId)
						missing := mkMissingPlayers(4 - len(current.Players))
						attach := fmt.Sprintf("<!here> The game needs %s more players", strings.Join(missing, ""))
						mkSlackResp(w, msg, []string{attach})
					}
				} else {
					log.Printf("User [%s] already signed up for the current game", user)
					log.Printf("[WaitingForPlayers] players: %v", current.Players)
					msg := fmt.Sprintf("<@%s> you have already been added to the current game", userId)
					missing := mkMissingPlayers(4 - len(current.Players))
					attach := fmt.Sprintf("<!here> The game needs %s more players", strings.Join(missing, ""))
					mkSlackResp(w, msg, []string{attach})
				}
			} else if command == GiveUp {
				if ok {
					log.Printf("Removing [%s] from the current game", user)
					delete(current.Players, userId)
					log.Printf("[WaitingForPlayers] players: %v", current.Players)
					if len(current.Players) == 0 {
						msg := fmt.Sprintf("<!here> <@%s> Just abandoned the game", userId)
						mkSlackResp(w, msg, []string{"No players left: game has been canceled!"})
						current = Status{
							State: NoGame,
							Players: map[string]int{},
						}
					} else {
						msg := fmt.Sprintf("<!here> <@%s> Just abandoned the game", userId)
						missing := mkMissingPlayers(4 - len(current.Players))
						attach := fmt.Sprintf("<!here> The game needs %s more players", strings.Join(missing, ""))
						mkSlackResp(w, msg, []string{attach})
					}
				} else {
					log.Printf("User [%s] never signed up for the current game", user)
					log.Printf("[WaitingForPlayers] players: %v", current.Players)
					msg := fmt.Sprintf("<@%s> you are not in the current game", userId)
					mkSlackResp(w, msg, []string{})
				}
			} else if command == Reset {
				mkSlackResp(w, "<!here> Game has been canceled!", []string{})
				current = Status{
					State: NoGame,
					Players: map[string]int{},
				}
			} else if command == Explain {
				fmt.Fprintf(w, "%s", ExplainMessage)
			} else if command == New {
				log.Printf("[WaitingForPlayers] Game is already created")
				log.Printf("[WaitingForPlayers] players: %v", current.Players)
				msg := "Game already created"
				missing := mkMissingPlayers(4 - len(current.Players))
				attach := fmt.Sprintf("<!here> The game needs %s more players", strings.Join(missing, ""))
				mkSlackResp(w, msg, []string{attach})
			} else if command == Current {
				log.Printf("[WaitingForPlayers] Asked for stats")
				players := current.getPlayers()
				mkSlackResp(w, fmt.Sprintf("Current players: [%s]", strings.Join(players, ", ")), []string{"Use */play* to join"})
			} else {
				log.Printf("[WaitingForPlayers] Unrecognized command: [%s]", command)
				log.Printf("[WaitingForPlayers] players: %v", current.Players)
				mkSlackResp(w, "Unrecognized command", []string{"Use */explain* for a list of all available commands"})
			}
		default:
			log.Fatalf("Unrecognized status")
			fmt.Fprintf(w, "WFH")
		}
	})

	log.Fatal(http.ListenAndServe(":9000", nil))
}
