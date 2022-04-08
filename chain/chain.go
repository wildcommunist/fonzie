package chain

import (
	"context"
	"fmt"
	"os"

	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	resty "github.com/go-resty/resty/v2"
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
	chainID, err := getChainID(chain.RPC)
	if err != nil {
		return err
	}

	// Build chain config
	chainConfig_1 := lens.ChainClientConfig{
		Key:     "anon",
		ChainID: chainID,
		RPCAddr: chain.RPC,
		// GRPCAddr       string,
		AccountPrefix:  chain.Prefix,
		KeyringBackend: "memory",
		GasAdjustment:  1.2,
		// GasPrices:      "0.01uosmo",
		KeyDirectory: "",
		Debug:        true,
		Timeout:      "20s",
		OutputFormat: "json",
		SignModeStr:  "direct",
		Modules:      lens.ModuleBasics,
	}

	// Creates client object to pull chain info
	chainClient, err := lens.NewChainClient(&chainConfig_1, "", os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("Failed to build new chain client for %s. Err: %v", toAddr, err)
	}

	// Lets restore a key with funds and name it "source_key", this is the wallet we'll use to send tx.
	faucetAddr, err := chainClient.RestoreKey("anon", mnemonic)
	if err != nil {
		return err
	}
	log.Infof("Sending %s from faucet address [%s] to recipient [%s]", coins, faucetAddr, toAddr)

	//	Now that we know our key name, we can set it in our chain config
	chainConfig_1.Key = "anon"

	//	Build transaction message
	req := &banktypes.MsgSend{
		FromAddress: faucetAddr,
		ToAddress:   toAddr,
		Amount:      coins,
	}

	// Send message and get response
	res, err := chainClient.SendMsg(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Println(chainClient.PrintTxResponse(res))

	return nil
}

func getChainID(rpcUrl string) (string, error) {
	rpc := resty.New().SetHostURL(rpcUrl)

	resp, err := rpc.R().
		SetResult(map[string]interface{}{}).
		SetError(map[string]interface{}{}).
		Get("/commit")
	if err != nil {
		return "", err
	}

	if resp.IsError() {
		//return "", resp.Error().(*map[string]interface{})
		return "", fmt.Errorf("could not get chain id; http error code received %d", resp.StatusCode())
	}

	respbody := resp.Result().(*map[string]interface{})
	result := (*respbody)["result"]
	signedHeader := result.(map[string]interface{})["signed_header"]
	header := signedHeader.(map[string]interface{})["header"]
	chainID := header.(map[string]interface{})["chain_id"].(string)
	return chainID, nil
}

/*
"result": {
	"signed_header": {
	  "header": {
	    "version": {
	      "block": "11"
	    },
	    "chain_id": "umee-1",
	    "height": "731426",
*/
