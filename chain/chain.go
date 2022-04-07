package chain

import (
	"context"

	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	log "github.com/sirupsen/logrus"
	"github.com/tendermint/starport/starport/pkg/cosmosclient"
)

type Chains []*Chain

func (chains Chains) ImportMnemonic(ctx context.Context, mnemonic string) error {
	for _, info := range chains {
		err := info.ImportMnemonic(ctx, mnemonic)
		if err != nil {
			return err
		}
	}
	return nil
}

func (chains Chains) FindByPrefix(prefix string) *Chain {
	for _, info := range chains {
		if info.Prefix == prefix {
			return info
		}
	}
	return nil
}

type Chain struct {
	Prefix string               `json:"prefix"`
	RPC    string               `json:"rpc"`
	client *cosmosclient.Client `json:"-"`
}

func (chain *Chain) getClient(ctx context.Context) cosmosclient.Client {
	if chain.client == nil {
		c, err := cosmosclient.New(ctx,
			cosmosclient.WithKeyringBackend("memory"),
			cosmosclient.WithNodeAddress(chain.RPC),
			cosmosclient.WithAddressPrefix(chain.Prefix),
		)
		if err != nil {
			log.Fatal(err)
		}

		chain.client = &c
	}
	return *chain.client
}

func (chain *Chain) ImportMnemonic(ctx context.Context, mnemonic string) error {
	c := chain.getClient(ctx)

	_, err := c.AccountRegistry.Import("anon", mnemonic, "")
	if err != nil {
		return err
	}

	return nil
}

func (chain Chain) Send(ctx context.Context, toAddr string, coins cosmostypes.Coins) error {
	c := chain.getClient(ctx)

	faucet, err := c.AccountRegistry.GetByName("anon")
	if err != nil {
		return err
	}
	faucetAddr := faucet.Address(chain.Prefix)

	msg := &banktypes.MsgSend{FromAddress: faucetAddr, ToAddress: toAddr, Amount: coins}
	log.Debugf("MSG: %#v\n", msg)

	_, err = c.BroadcastTx("anon", msg)
	if err != nil {
		return err
	}

	return nil
}
