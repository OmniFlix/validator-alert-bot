package main

import (
	"encoding/json"
	"fmt"
	"github.com/anakin0xc06/validator-alert-bot/config"
	"github.com/anakin0xc06/validator-alert-bot/helpers"
	"github.com/jasonlvhit/gocron"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/bech32"
	"github.com/fatih/color"
)

var subscribers = make(map[string][]string)
var validatorsMissedBlocks = make(map[string]int64)
var networks = make(map[string]map[string]string)

func initBot() {
	jsonFile, err := os.Open(config.SubscribersFile)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &subscribers)
	fmt.Println(subscribers)
	validatorsfile, err := os.Open(config.ValidatorsFile)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	defer validatorsfile.Close()
	byteValue2, _ := ioutil.ReadAll(validatorsfile)
	json.Unmarshal(byteValue2, &validatorsMissedBlocks)
	networksfile, err := os.Open(config.NetworksFile)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	defer validatorsfile.Close()
	byteValue3, _ := ioutil.ReadAll(networksfile)
	json.Unmarshal(byteValue3, &networks)
	UpdateValidatorMissedBlocks()
	time.Sleep(5)
}

func main() {
	bot, err := tgbotapi.NewBotAPI(config.BOT_API_KEY)
	if err != nil {
		log.Fatalf("Error in instantiating the bot: %v", err)
	}
	initBot()
	go SubscribersHandleScheduler(bot)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		color.Red("Error while receiving messages: %s", err)
		return
	}
	color.Green("Started %s successfully", bot.Self.UserName)

	for update := range updates {
		if update.Message != nil && update.Message.IsCommand() {
			MainHandler(bot, update)
		}
	}
}

// MainHandler ...
func MainHandler(bot *tgbotapi.BotAPI, update tgbotapi.Update) {

	if update.Message != nil && update.Message.IsCommand() && update.Message.Chat.IsPrivate() {
		command := update.Message.Command()

		switch command {
		case "start":
			text := "Welcome to validator-alerts bot\n"
			helpers.SendMessage(bot, update, text, "html")
		case "subscribe":
			HandleSubscribe(bot, update)
		case "unsubscribe":
			HandleUnsubscribe(bot, update)
		default:
			text := "Command not available"
			fmt.Println(command, text)
			// helpers.SendMessage(bot, update, text, "html")
		}
	}
}

func HandleSubscribe(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	args := update.Message.CommandArguments()
	var validatorConsAddresses []string

	if len(args) > 0 {
		arguments := strings.Split(args, " ")
		for _, arg := range arguments {
			if isCorrectValConsAddress(arg) && !contains(validatorConsAddresses, arg) {
				validatorConsAddresses = append(validatorConsAddresses, arg)
			}
		}

		if len(validatorConsAddresses) > 0 {
			userId := helpers.GetUserID(update)
			validators, ok := subscribers[fmt.Sprint(userId)]
			if !ok {
				subscribers[fmt.Sprint(userId)] = validatorConsAddresses
			} else {
				for _, val := range validatorConsAddresses {
					if !contains(validators, val) {
						validators = append(validators, val)
					}
				}
				subscribers[fmt.Sprint(userId)] = validators
			}
			jsonString, _ := json.MarshalIndent(subscribers, "", " ")
			_ = ioutil.WriteFile(config.SubscribersFile, jsonString, 0644)
			helpers.SendMessage(bot, update, "subscribed to alerts.", tgbotapi.ModeHTML)
			return
		} else {
			helpers.SendMessage(bot, update, "Invalid args", tgbotapi.ModeHTML)
			return
		}

	} else {
		helpers.SendMessage(bot, update, "Invalid format, Please use /subscribe [validator consensus addresses ..]", tgbotapi.ModeHTML)
		return
	}
}

func HandleUnsubscribe(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	userId := helpers.GetUserID(update)
	_, ok := subscribers[fmt.Sprint(userId)]
	if ok {
		delete(subscribers, fmt.Sprint(userId))
	}
	data, _ := json.MarshalIndent(subscribers, "", " ")
	_ = ioutil.WriteFile(config.SubscribersFile, data, 0644)
	text := "unsubscribed from alerts"
	helpers.SendMessage(bot, update, text, tgbotapi.ModeHTML)
}

func UpdateValidatorMissedBlocks() {
	var addresses []string
	for _, validators := range subscribers {
		for _, validator := range validators {
			if !contains(addresses, validator) {
				addresses = append(addresses, validator)
			}
		}
	}
	for _, address := range addresses {
		prefix := getPrefix(address)
		if len(prefix) > 0 && len(networks[prefix]) > 0 {
			missedCount, err := helpers.CheckMissedBlocks(networks[prefix]["rest"], address)
			if err != nil {
				continue
			}
			validatorsMissedBlocks[address] = missedCount
		}
	}
	validatorsData, _ := json.MarshalIndent(validatorsMissedBlocks, "", " ")
	_ = ioutil.WriteFile(config.ValidatorsFile, validatorsData, 0644)
	fmt.Println("Updated validators missed blocks data")
}

func getPrefix(addr string) string {
	parts := strings.Split(addr, "val")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}
func HandleSubscribers(bot *tgbotapi.BotAPI) {
	fmt.Println("Checking Missed blocks ...")
	for user, validators := range subscribers {
		for _, validator := range validators {
			prefix := getPrefix(validator)
			fmt.Println(validator)
			if len(prefix) > 0 && len(networks[prefix]) > 0 {
				currentMissedBlocks, err := helpers.CheckMissedBlocks(networks[prefix]["rest"], validator)
				if err != nil {
					continue
				}
				fmt.Println("Missed Blocks:", currentMissedBlocks)
				fmt.Printf("\n")
				previousMissedBlocks, ok := validatorsMissedBlocks[validator]
				if !ok {
					continue
				}
				if currentMissedBlocks-previousMissedBlocks > config.MissedBlocksLimit {
					fmt.Println(validator, "missing blocks")
					text := fmt.Sprintf("**Alert**:\n\n %s is missing blocks MissedBlocksCount **%d -> %d**",
						validator, previousMissedBlocks, currentMissedBlocks)
					userId, _ := strconv.ParseInt(user, 10, 64)
					msg := tgbotapi.NewMessage(userId, text)
					msg.ParseMode = tgbotapi.ModeMarkdown
					bot.Send(msg)
				}
			}
		}
	}
	UpdateValidatorMissedBlocks()
}

func isCorrectValConsAddress(address string) bool {
	hrp, _, err := bech32.Decode(address)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	if strings.Contains(hrp, "valcons") {
		return true
	}
	return false
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func SubscribersHandleScheduler(bot *tgbotapi.BotAPI) {
	HandleSubscribers(bot)
	time.Sleep(5 * time.Second)
	s := gocron.NewScheduler()
	fmt.Println("Starting blocks monitoring scheduler ...")
	s.Every(60).Seconds().Do(HandleSubscribers, bot)
	<-s.Start()
}
