package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/Entrio/subenv"
	"github.com/bwmarrin/discordgo"
	"github.com/cosmos/cosmos-sdk/types"
	log "github.com/sirupsen/logrus"

	"github.com/umee-network/fonzie/chain"
)

/*
 * 1. create a worker -> a go routine which will consume a channel
 * 2. the worker will wait for new requests and have a time guard for processing faucet requests
 *   - we will batch requests in 4s, max 50 requests per transactions
 *
 */

type (
	FaucetReq struct {
		Recipient types.AccAddress
		Coins     types.Coins
		Fees      types.Coins
		session   *discordgo.Session
		msg       *discordgo.MessageCreate
	}
	StatusReq struct {
		session *discordgo.Session
		msg     *discordgo.MessageCreate
	}
)

type BalanceResponse struct {
	Balances []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"balances"`
	Pagination struct {
		NextKey string `json:"next_key"`
		Total   string `json:"total"`
	} `json:"pagination"`
}

type ChainFaucet struct {
	channel chan FaucetReq
	status  chan StatusReq
	chain   *chain.Chain
}

func (br BalanceResponse) getBalance() string {
	for _, d := range br.Balances {
		if d.Denom == subenv.Env("BALANCE_DENOM", "uandr") {
			famt, err := strconv.ParseFloat(d.Amount, 64)
			if err != nil {
				return "error"
			}
			return fmt.Sprintf("%f", famt/1000000)
		}
	}
	return "NaN"
}

func (cf ChainFaucet) Consume(quit chan bool) {
	log.Info("starting worker ", cf.chain.Prefix)
	var r FaucetReq
	var rs []FaucetReq
	const interval = time.Second * 1
	var t = time.NewTicker(interval)

	for {
		select {
		case r = <-cf.channel:
			log.Infof("%s worker NEW request, req: %v", cf.chain.Prefix, r)
			rs = append(rs, r)
			if len(rs) > 160 { // Maximum of 160 wallets in single multisend
				cf.processRequests(rs)
				rs = make([]FaucetReq, 0)
				t.Reset(interval)
			} else {
				log.Infof("%s worker waiting for more requests, %v", cf.chain.Prefix, r)
			}
		case sr := <-cf.status:
			cf.processStatusRequests(sr)
		case <-t.C:
			if len(rs) > 0 {
				cf.processRequests(rs)
				rs = make([]FaucetReq, 0)
			}

		case <-quit:
			if len(rs) > 0 {
				cf.processRequests(rs)
			}
			// die so kubernetes restarts pod
			log.Fatal("Worker ", cf.chain.Prefix, " quit")
		}
	}
}

func (cf ChainFaucet) processStatusRequests(sr StatusReq) {
	c := cf.chain.GetClient()
	faucetRawAddr, err := c.GetKeyAddress()
	if err != nil {
		reportError(sr.session, sr.msg, err)
		return
	}
	faucetAddrStr, err := c.EncodeBech32AccAddr(faucetRawAddr)
	if err != nil {
		reportError(sr.session, sr.msg, err)
		return
	}
	url := fmt.Sprintf("%s/cosmos/bank/v1beta1/balances/%s", subenv.Env("LCD_ADDRESS", "http://127.0.0.1:1317"), faucetAddrStr)
	client := http.Client{Timeout: time.Second * 2}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		reportError(sr.session, sr.msg, fmt.Errorf("failed to create request"))
		log.Error(err.Error())
		return
	}
	req.Header.Set("User-Agent", "andromeda-fonzie-ayeee")

	res, getErr := client.Do(req)
	if getErr != nil {
		reportError(sr.session, sr.msg, fmt.Errorf("failed to query LCD"))
		log.Error(err.Error())
		return
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		reportError(sr.session, sr.msg, fmt.Errorf("failed to read LCD response"))
		log.Error(err.Error())
		return
	}

	response := BalanceResponse{}
	jsonErr := json.Unmarshal(body, &response)
	if jsonErr != nil {
		reportError(sr.session, sr.msg, fmt.Errorf("failed to parse LCD response"))
		log.Error(err.Error())
		return
	}
	removedReaction(sr.session, sr.msg, "⚙️")
	sendReaction(sr.session, sr.msg, "✅")
	_, err = sr.session.ChannelMessageSendReply(sr.msg.ChannelID,
		fmt.Sprintf("Faucet status:\nCurrent balance: `%s ANDR`\nSend DMs: `%v`", response.getBalance(), subenv.EnvB("SEND_DM", false)),
		sr.msg.Reference())
	if err != nil {
		log.Error(err)
	}
}

func (cf ChainFaucet) processRequests(rs []FaucetReq) {
	var toAddrss = make([]types.AccAddress, 0, len(rs))
	var coins = make([]types.Coins, 0, len(rs))
	var fees = make(types.Coins, 0, len(rs))
	for _, r := range rs {
		toAddrss = append(toAddrss, r.Recipient)
		coins = append(coins, r.Coins)
		fees = fees.Add(r.Fees...)
	}
	err, txh := cf.chain.MultiSend(toAddrss, coins, fees)
	if err != nil {
		for _, r := range rs {
			reportError(r.session, r.msg, err)
		}
	} else {
		for _, r := range rs {
			// Everything worked, so-- respond successfully to Discord requester
			sendReaction(r.session, r.msg, "✅")
			removedReaction(r.session, r.msg, "⚙️")
			_, err = r.session.ChannelMessageSendReply(r.msg.ChannelID,
				fmt.Sprintf("Hey <@%s>, faucet tapped, just for you!\nTransaction hash\n%s/%s", r.msg.Author.ID, subenv.Env("FINDER_URL", "https://ping.wildsage.io/andromeda/tx"), txh),
				r.msg.Reference())
			if err != nil {
				log.Error(err)
			}
			if subenv.EnvB("SEND_DM", false) {
				sendMessage(r.session, r.msg, fmt.Sprintf("Dispensed 💸 `%s` to `%s`\n%s", r.Coins, r.Recipient, fmt.Sprintf("Transaction hash\n%s/%s", subenv.Env("FINDER_URL", "https://ping.wildsage.io/andromeda/tx"), txh)))
			}

		}
	}
}
