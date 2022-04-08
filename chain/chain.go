package chain

import (
	"context"
	"fmt"
	"os"

	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	log "github.com/sirupsen/logrus"
	lens "github.com/strangelove-ventures/lens/client"
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
	log.Infof("Sending %s from faucet address [%s] to recipient [%s]", coins, faucetAddr, toAddr)

	msg := &banktypes.MsgSend{FromAddress: faucetAddr, ToAddress: toAddr, Amount: coins}
	log.Debugf("MSG: %#v\n", msg)

	_, err = c.BroadcastTx("anon", msg)
	if err != nil {
		return err
	}

	return nil
}

func (chain Chain) SendLense(toAddr string, coins cosmostypes.Coins, mnemonic string) error {
	// For this example, lets place the key directory in your PWD.
	pwd, _ := os.Getwd()
	key_dir := pwd + "/keys"

	// faucetAddr := faucet.Address(chain.Prefix)
	// log.Infof("Sending %s from faucet address [%s] to recipient [%s]", coins, faucetAddr, toAddr)
	// msg := &banktypes.MsgSend{FromAddress: faucetAddr, ToAddress: toAddr, Amount: coins}

	// Build chain config
	chainConfig_1 := lens.ChainClientConfig{
		Key:     "default",
		ChainID: "umeemania-1",
		RPCAddr: chain.RPC,
		// GRPCAddr       string,
		AccountPrefix:  chain.Prefix,
		KeyringBackend: "memory",
		GasAdjustment:  1.2,
		// GasPrices:      "0.01uosmo",
		KeyDirectory: key_dir,
		Debug:        true,
		Timeout:      "20s",
		OutputFormat: "json",
		SignModeStr:  "direct",
		Modules:      lens.ModuleBasics,
	}

	// Creates client object to pull chain info
	chainClient, err := lens.NewChainClient(&chainConfig_1, key_dir, os.Stdin, os.Stdout)
	if err != nil {
		log.Fatalf("Failed to build new chain client for %s. Err: %v \n", toAddr, err)
	}

	// Lets restore a key with funds and name it "source_key", this is the wallet we'll use to send tx.
	srcWalletAddress, err := chainClient.RestoreKey("source_key", mnemonic)
	if err != nil {
		log.Fatalf("Failed to restore key. Err: %v \n", err)
	}

	//	Now that we know our key name, we can set it in our chain config
	chainConfig_1.Key = "source_key"

	//	Build transaction message
	req := &banktypes.MsgSend{
		FromAddress: srcWalletAddress,
		ToAddress:   toAddr,
		Amount:      coins,
	}

	// Send message and get response
	res, err := chainClient.SendMsg(context.Background(), req)
	if err != nil {
		if res != nil {
			log.Fatalf("failed to send coins: code(%d) msg(%s)", res.Code, res.Logs)
		}
		log.Fatalf("Failed to send coins.Err: %v", err)
	}
	fmt.Println(chainClient.PrintTxResponse(res))

	return nil
}
