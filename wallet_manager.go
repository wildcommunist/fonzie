package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/umee-network/fonzie/chain"
)

/*
 * 1. create a worker -> a go routine which will consume a channel
 * 2. the worker will wait for new requests and have a time guard for processing faucet requests
 *   - we will batch requests in 4s, max 50 requests per transactions
 *
 */

type FaucetReq struct {
	Recipient types.AccAddress
	Coins     types.Coins
	session   *discordgo.Session
	msg       *discordgo.MessageCreate
}

type ChainFaucet struct {
	channel chan FaucetReq
	chain   *chain.Chain
}

func (cf ChainFaucet) Consume(quit chan bool) {
	var r FaucetReq
	var rs []FaucetReq
	const interval = time.Second * 4
	var t = time.NewTicker(interval)

	for {
		select {
		case r = <-cf.channel:
			rs = append(rs, r)
			if len(rs) > 20 {
				cf.processRequests(rs)
				rs = make([]FaucetReq, 0)
				t.Reset(interval)
			}
		case <-t.C:
			if len(rs) > 0 {
				cf.processRequests(rs)
				rs = make([]FaucetReq, 0)
			}

		case <-quit:
			if len(rs) > 0 {
				cf.processRequests(rs)
			}
			return
		}
	}
}

func (cf ChainFaucet) processRequests(rs []FaucetReq) {
	var toAddrss = make([]types.AccAddress, 0, len(rs))
	var coins = make([]types.Coins, 0, len(rs))
	for _, r := range rs {
		toAddrss = append(toAddrss, r.Recipient)
		coins = append(coins, r.Coins)
	}
	err := cf.chain.MultiSend(toAddrss, coins)
	if err != nil {
		for _, r := range rs {
			reportError(r.session, r.msg, err)
		}
	} else {
		for _, r := range rs {
			// Everything worked, so-- respond successfully to Discord requester
			sendReaction(r.session, r.msg, "✅")
			sendMessage(r.session, r.msg, fmt.Sprintf("Dispensed 💸 `%s`",  r.Coins))
		}
	}
}