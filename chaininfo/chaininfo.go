package chaininfo

import (
	"context"

	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	log "github.com/sirupsen/logrus"
	"github.com/tendermint/starport/starport/pkg/cosmosclient"
)

type ChainInfos []ChainInfo

func (infos ChainInfos) FindByPrefix(prefix string) *ChainInfo {
	for _, info := range infos {
		if info.Prefix == prefix {
			return &info
		}
	}
	return nil
}

type ChainInfo struct {
	Prefix string               `json:"prefix"`
	RPC    string               `json:"rpc"`
	client *cosmosclient.Client `json:"-"`
}

func (info *ChainInfo) getClient(ctx context.Context) cosmosclient.Client {
	if info.client == nil {
		c, err := cosmosclient.New(ctx,
			cosmosclient.WithKeyringBackend("memory"),
			cosmosclient.WithNodeAddress(info.RPC),
			cosmosclient.WithAddressPrefix(info.Prefix),
		)
		if err != nil {
			log.Fatal(err)
		}

		info.client = &c
	}
	return *info.client
}

func (info *ChainInfo) ImportMnemonic(ctx context.Context, mnemonic string) error {
	chain := info.getClient(ctx)

	_, err := chain.AccountRegistry.Import("anon", mnemonic, "")
	if err != nil {
		return err
	}

	return nil
}

func (info ChainInfo) Send(ctx context.Context, toAddr string, coins cosmostypes.Coins) error {
	chain := info.getClient(ctx)

	faucet, err := chain.AccountRegistry.GetByName("anon")
	if err != nil {
		return err
	}
	faucetAddr := faucet.Address(info.Prefix)

	msg := &banktypes.MsgSend{FromAddress: faucetAddr, ToAddress: toAddr, Amount: coins}
	log.Debugf("MSG: %#v\n", msg)

	_, err = chain.BroadcastTx("anon", msg)
	if err != nil {
		return err
	}

	return nil
}
