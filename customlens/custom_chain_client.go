package customlens

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	lens "github.com/strangelove-ventures/lens/client"
)

type CustomChainClient struct {
	*lens.ChainClient
}

// SendMsg yeet
func (cc *CustomChainClient) SendMsg(ctx context.Context, msg sdk.Msg, fees string) (*sdk.TxResponse, error) {
	return cc.SendMsgs(ctx, []sdk.Msg{msg}, fees)
}

// SendMsgs yeet
func (cc *CustomChainClient) SendMsgs(ctx context.Context, msgs []sdk.Msg, fees string) (*sdk.TxResponse, error) {
	txf, err := cc.PrepareFactory(cc.TxFactory())
	if err != nil {
		return nil, err
	}

	// TODO: Make this work with new CalculateGas method
	// TODO: This is related to GRPC client stuff?
	// https://github.com/cosmos/cosmos-sdk/blob/5725659684fc93790a63981c653feee33ecf3225/client/tx/tx.go#L297
	_, adjusted, err := cc.ChainClient.CalculateGas(txf, msgs...)
	if err != nil {
		return nil, err
	}

	// Set the gas amount on the transaction factory
	txf = txf.WithGas(adjusted)

	// Set the fees, if they exist
	if fees != "" {
		txf.WithFees(fees)
	}

	// Build the transaction builder
	txb, err := tx.BuildUnsignedTx(txf, msgs...)
	if err != nil {
		return nil, err
	}

	// Attach the signature to the transaction
	// c.LogFailedTx(nil, err, msgs)
	// Force encoding in the chain specific address
	for _, msg := range msgs {
		cc.Codec.Marshaler.MustMarshalJSON(msg)
	}

	err = func() error {
		done := cc.SetSDKContext()
		// ensure that we allways call done, even in case of an error or panic
		defer done()
		if err = tx.Sign(txf, cc.Config.Key, txb, false); err != nil {
			return err
		}
		return nil
	}()

	if err != nil {
		return nil, err
	}

	// Generate the transaction bytes
	txBytes, err := cc.Codec.TxConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return nil, err
	}

	// Broadcast those bytes
	res, err := cc.BroadcastTx(ctx, txBytes)
	if err != nil {
		return nil, err
	}

	// transaction was executed, log the success or failure using the tx response code
	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.
	if res.Code != 0 {
		return res, fmt.Errorf("transaction failed with code: %d", res.Code)
	}

	return res, nil
}
