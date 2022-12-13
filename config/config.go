package config

import "os"

var (
	BOT_API_KEY     = os.Getenv("BOT_API_KEY")
	SubscribersFile = "./data/subscribers.json"
	ValidatorsFile  = "./data/validators.json"
	NetworksFile    = "./config/networks.json"
)

const (
	MissedBlocksLimit = 10
)
