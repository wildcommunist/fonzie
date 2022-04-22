package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/cosmos/btcutil/bech32"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	log "github.com/sirupsen/logrus"
	chain "github.com/umee-network/fonzie/chain"
)

//go:generate bash -c "if [ \"$CI\" = true ] ; then echo -n $GITHUB_REF_NAME > VERSION; fi"
var (
	//go:embed VERSION
	Version string
)

type CoinsStr = string
type ChainFunding = map[ChainPrefix]CoinsStr

var (
	mnemonic   = os.Getenv("MNEMONIC")
	botToken   = os.Getenv("BOT_TOKEN")
	rawChains  = os.Getenv("CHAINS")
	rawFunding = os.Getenv("FUNDING")
	isSilent   = os.Getenv("SILENT") != ""
	funding    ChainFunding
)

func initChains() chain.Chains {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(Version)
		os.Exit(0)
	}

	log.SetFormatter(&log.JSONFormatter{})

	if mnemonic == "" {
		log.Fatal("MNEMONIC is invalid")
	}
	if botToken == "" {
		log.Fatal("BOT_TOKEN is invalid")
	}
	if rawChains == "" {
		log.Fatal("CHAINS config cannot be blank (json array)")
	}
	// parse chains config
	var chains chain.Chains
	err := json.Unmarshal([]byte(rawChains), &chains)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("CHAINS: %#v", chains)
	// parse chains config
	err = json.Unmarshal([]byte(rawFunding), &funding)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("CHAIN_FUNDING: %#v", funding)
	return chains
}

func main() {
	ctx := context.Background()
	db := NewDb(ctx)
	chains := initChains()

	err := chains.ImportMnemonic(ctx, mnemonic)
	if err != nil {
		log.Fatal(err)
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}
	defer dg.Close()

	fh := NewFaucetHandler(chains, db)
	dg.AddHandler(fh.handleDispense)

	// we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentDirectMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Fatal(err)
	}

	// Wait here until CTRL-C or other term signal is received.
	if isSilent {
		log.Info("SILENT MODE:  The Fonz is still running, only reporting errors.  Press CTRL-C to exit.")
	} else {
		log.Info("The Fonz bot is now thumbs-up'ing.  Press CTRL-C to exit.")
	}
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

type FaucetHandler struct {
	faucets map[string]ChainFaucet
	quit    chan bool
	chains  chain.Chains
	db      Db
	ctx     context.Context

	cmd *regexp.Regexp
}

func NewFaucetHandler(chains chain.Chains, db Db) FaucetHandler {
	re, err := regexp.Compile("!(request|help)(.*)")
	if err != nil {
		log.Fatal(err)
	}
	var faucets = make(map[string]ChainFaucet)
	var quit = make(chan bool)
	for _, c := range chains {
		f := ChainFaucet{make(chan FaucetReq), c}
		faucets[c.Prefix] = f
		go f.Consume(quit)
	}
	return FaucetHandler{
		faucets: faucets,
		quit:    quit,
		chains:  chains,
		cmd:     re,
		ctx:     context.Background(),
		db:      db,
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func (fh FaucetHandler) handleDispense(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Do we support this bech32 prefix?
	matches := fh.cmd.FindAllStringSubmatch(m.Content, -1)
	if len(matches) > 0 {
		// for each matched request, do--
		for _, match := range matches {
			cmd := strings.TrimSpace(match[1])
			args := strings.TrimSpace(match[2])
			switch cmd {
			case "request":
				// TODO if role doesn't exist, reply with help and return
				// - "umeemaniac"
				// - ROLE_REQUIRED="role string/id", optional from env
				dstAddr := args
				prefix, _, err := bech32.Decode(dstAddr, 1023)
				if err != nil {
					reportError(s, m, err)
					return
				}

				faucet, ok := fh.faucets[prefix]
				if !ok {
					reportError(s, m, fmt.Errorf("%s chain prefix is not supported", prefix))
					return
				}
				coins, err := cosmostypes.ParseCoinsNormalized(funding[prefix])
				if err != nil {
					reportError(s, m, fmt.Errorf("%s chain prefix is not supported, err: %v", prefix, err))
					log.Error(err)
					return
				}

				// TODO refactor Prune into FindByChainPrefixAndUsername?
				maxAge := time.Hour * 12
				receipt, err := fh.db.GetFundingReceiptByUsernameAndChainPrefix(fh.ctx, prefix, m.Author.Username)
				if err != nil {
					reportError(s, m, err)
					return
				}
				if receipt != nil && receipt.FundedAt.Add(maxAge).Before(time.Now()) {
					reportError(s, m, fmt.Errorf("you must wait %v until you can get %s funding again", time.Until(receipt.FundedAt.Add(maxAge)).Round(2*time.Second), prefix))
					return
				}

				recipient, err := faucet.chain.DecodeAddr(dstAddr)
				if err != nil {
					reportError(s, m, fmt.Errorf("malformed destination address, err: %w", err))
					return
				}

				// Immediately respond to Discord
				sendReaction(s, m, "👍")
				faucet.channel <- FaucetReq{recipient, coins, s, m}

				err = fh.db.SaveFundingReceipt(fh.ctx, FundingReceipt{
					ChainPrefix: prefix,
					Username:    m.Author.Username,
					FundedAt:    time.Now(),
					Amount:      coins,
				})
				if err != nil {
					log.Error(err)
				}

			default:
				help(s, m, fh.chains)
			}
		}
	} else if m.GuildID == "" {
		// If message is DM, respond with help
		help(s, m, fh.chains)
	}
}

func reportError(s *discordgo.Session, m *discordgo.MessageCreate, errToReport error) {
	if m.Author.Bot {
		// guard against known bots
		return
	}
	err := sendReaction(s, m, "❌")
	if err != nil {
		log.Error(err)
	}
	// Send errors to channel, even when isSilent
	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s>, there is an error in your request:\n `%s`", m.Author.ID, errToReport))
	if err != nil {
		log.Error(err)
	}
}

//go:embed help.md
var helpMsg string

func help(s *discordgo.Session, m *discordgo.MessageCreate, chains chain.Chains) {
	acc := []string{}
	for _, chain := range chains {
		acc = append(acc, chain.Prefix)
	}
	err := sendMessage(s, m, fmt.Sprintf("**Supported address prefixes**: %s.\n\n%s", strings.Join(acc, ", "), helpMsg))
	if err != nil {
		log.Error(err)
	}
}

func sendMessage(s *discordgo.Session, m *discordgo.MessageCreate, msg string) error {
	if isSilent || m.Author.Bot {
		return nil
	}
	directMessageChannel, err := s.UserChannelCreate(m.Author.ID)
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSend(directMessageChannel.ID, msg)
	if err != nil {
		return err
	}
	return nil
}

func sendReaction(s *discordgo.Session, m *discordgo.MessageCreate, reaction string) error {
	if isSilent || m.Author.Bot {
		return nil
	}
	return s.MessageReactionAdd(m.ChannelID, m.ID, reaction)
}
