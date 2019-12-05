package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/dehimb/piperunner/pkg/turnpike"
	. "github.com/logrusorgru/aurora"
)

type Config struct {
	UserID   string `json:"userId"`
	BotToken string `json:"botToken"`
}

type Assert struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Value string `json:"value"`
}

type Response struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Value string `json:"value"`
}

type Step struct {
	Asserts  []Assert `json:"asserts"`
	Response Response `json:"response"`
}

type Test struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

type Runner struct {
	Config Config `json:"config"`
	Tests  []Test `json:"tests"`
}

type State struct {
	Messages []string
	Buttons  []Button
}

type Message struct {
	Type         string `json:"type"`
	Text         string `json:"text"`
	ReplyPayload string `json:"reply_payload"`
}

type User struct {
	ExtID string `json:"ext_id"`
}

type Package struct {
	User    User    `json:"user"`
	Message Message `json:"message"`
	Token   string  `json:"token"`
}

type Button struct {
	ReplyPayload string `json:"reply_payload"`
	Text         string `json:"text"`
}

func main() {
	var runner = Runner{}
	var state State
	file, _ := ioutil.ReadFile("tests.json")
	err := json.Unmarshal([]byte(file), &runner)
	if err != nil {
		fmt.Printf("Can't read json file: %s", Red(err))
		os.Exit(1)
	}
	c := turnpike.NewClient()
	if err = c.Connect("wss://pipe.bot:2443", "https://pipe.bot"); err != nil {
		fmt.Println(Red("Websocket connection error"))
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("%s %s\n", Cyan("WEBSOCKET CONNECTION STATUS"), Green("OK"))
	channel := make(chan interface{})
	topic := fmt.Sprintf("chat-%s-%s", runner.Config.BotToken, runner.Config.UserID)
	err = c.Subscribe(topic, func(topic string, event interface{}) {
		data := event.(map[string]interface{})
		switch data["type"] {
		case "text":
			state.Messages = append(state.Messages, fmt.Sprintf("%v", data["text"]))
		case "buttons":
			state.Messages = append(state.Messages, fmt.Sprintf("%v", data["text"]))
			for _, button := range data["buttons"].([]interface{}) {
				item := button.(map[string]interface{})
				button := Button{
					ReplyPayload: fmt.Sprintf("%v", item["reply_payload"]),
					Text:         fmt.Sprintf("%v", item["text"]),
				}
				state.Buttons = append(state.Buttons, button)
			}
			channel <- "buttons"
		case "input":
			channel <- "input"
		default:
		}
	})
	if err != nil {
		fmt.Println(Red("Subscrition error"))
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("%s %s\n\n", Cyan("TOPIC SUBSCRIPTION STATUS"), Green("OK"))

	pkg := Package{
		User: User{
			ExtID: runner.Config.UserID,
		},
		Message: Message{},
		Token:   runner.Config.BotToken,
	}
	for _, test := range runner.Tests {
		start := time.Now()
		fmt.Printf("%s %s\n", Cyan("STARTING TEST"), Blue(test.Name))
		err := sendText(c, pkg, "/start")
		if err != nil {
			fmt.Println(Red("Can't send message /start"))
			fmt.Println(Red(err))
			os.Exit(1)
		}
		<-channel
		for k, step := range test.Steps {
			fmt.Printf("%s%d\n", Cyan("STEP #"), Cyan(k))
			for _, assert := range step.Asserts {
				switch assert.Type {
				case "text":
					if assert.Value != state.Messages[assert.Index] {
						fmt.Printf("%s %s %s %s %s\n", Red("FAIL"), Cyan("EXPECTED"), Blue(assert.Value), Cyan("RECEIVED"), Blue(state.Messages[assert.Index]))
						os.Exit(1)
					}
					fmt.Printf("%s %s %s\n", Green("OK"), Cyan("ASSERT TEXT"), Blue(assert.Value))
				case "button":
					if assert.Value != state.Buttons[assert.Index].Text {
						fmt.Printf("%s %s %s %s %s\n", Red("FAIL"), Cyan("EXPECTED"), Blue(assert.Value), Cyan("RECEIVED"), Blue(state.Buttons[assert.Index].Text))
						os.Exit(1)
					}
					fmt.Printf("%s %s %s\n", Green("OK"), Cyan("ASSERT BUTTON"), Blue(assert.Value))
				default:
				}
			}
			var payload string
			if step.Response.Type == "click" {
				payload = state.Buttons[step.Response.Index].ReplyPayload
			}
			state.Messages = state.Messages[:0]
			state.Buttons = state.Buttons[:0]
			if k < len(test.Steps)-1 {
				switch step.Response.Type {
				case "text":
					err := sendText(c, pkg, step.Response.Value)
					if err != nil {
						fmt.Printf("%s\n", "Can't send text")
						fmt.Printf("%s", err)
					}
				case "click":
					err := sendButtonClick(c, pkg, payload)
					if err != nil {
						fmt.Printf("%s\n", "Can't send button click")
						fmt.Printf("%s", err)
					}
				default:
				}
				<-channel
			}
		}
		elapsed := time.Since(start)
		fmt.Printf("%s %s %s %s %s %s\n\n", Cyan("END TEST"), Blue(test.Name), Cyan("STATUS"), Green("OK"), Cyan("TIME"), Cyan(elapsed))
	}

	err = c.Unsubscribe(topic)
	if err != nil {
		fmt.Printf("%s\n%s", Red("Error on unsubscribing"), err)
	}
	fmt.Println(Green("Done"))
}
func sendText(c *turnpike.Client, pkg Package, text string) error {
	pkg.Message = Message{
		Type: "text",
		Text: text,
	}
	data, err := json.Marshal(pkg)
	if err != nil {
		fmt.Println(Red("Can't marshal data"))
		fmt.Println(err)
		os.Exit(1)
	}
	return sendToSocket(c, data)
}

func sendButtonClick(c *turnpike.Client, pkg Package, payload string) error {
	pkg.Message = Message{
		Type:         "button",
		ReplyPayload: payload,
	}
	data, err := json.Marshal(pkg)
	if err != nil {
		fmt.Println(Red("Can't marshal data"))
		fmt.Println(err)
		os.Exit(1)
	}
	return sendToSocket(c, data)
}

func sendToSocket(c *turnpike.Client, data []byte) error {
	err := c.Publish("message.post", string(data))
	if err != nil {
		return err
	}
	return nil
}
