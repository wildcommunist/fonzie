package main

import (
	"context"
	"os"
	"time"

	b64 "encoding/base64"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ChainPrefix = string
type Username = string
type FundingReceipt struct {
	ChainPrefix ChainPrefix       `firestore:"chainPrefix"`
	Username    Username          `firestore:"username"`
	FundedAt    time.Time         `firestore:"fundedAt"`
	Amount      cosmostypes.Coins `firestore:"amount"`
}
type FundingReceipts []FundingReceipt

// Db represents the application interface for accessing the database
type Db struct {
	firestore *firestore.Client
}

func NewDb(ctx context.Context) Db {
	// Initialize Firestore
	client, err := initFirestore(ctx)
	if err != nil {
		log.Fatal(err)
	}
	return Db{
		firestore: client,
	}
}

// ProvideFirestore returns a *firestore.Client
func initFirestore(ctx context.Context) (*firestore.Client, error) {
	var (
		app  *firebase.App
		json []byte
		err  error
	)
	conf := &firebase.Config{ProjectID: os.Getenv("GCP_PROJECT")}
	if os.Getenv("GCP_CREDENTIALS") != "" {
		// import from env
		log.Info("Importing GCP credentials from env")
		json, err = b64.StdEncoding.DecodeString(os.Getenv("GCP_CREDENTIALS"))
		if err != nil {
			log.Fatal(err)
		}
		app, err = firebase.NewApp(ctx, conf, option.WithCredentialsJSON([]byte(json)))
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// local dev/application-default case
		app, err = firebase.NewApp(ctx, conf)
		if err != nil {
			log.Fatal(err)
		}
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		return nil, err
	}

	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		log.Println("🚨 Production Firestore Host (cloud) is activated")
	} else {
		log.Println("🧪 Emulator Firestore Host is activated")
	}
	return client, nil
}

func (db Db) SaveFundingReceipt(ctx context.Context, newReceipt FundingReceipt) error {
	table := db.firestore.Collection("funding-receipts")
	ref := table.Doc(newReceipt.Username + "." + newReceipt.ChainPrefix)

	_, err := ref.Set(ctx, newReceipt)
	return err
}

func (db Db) GetFundingReceiptByUsernameAndChainPrefix(ctx context.Context, username string, chainPrefix string) (*FundingReceipt, error) {
	table := db.firestore.Collection("funding-receipts")
	ref := table.Doc(username + "." + chainPrefix)

	doc, err := ref.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out FundingReceipt
	err = doc.DataTo(&out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

/*
func (receipts *FundingReceipts) Prune(maxAge time.Duration) {
	pruned := FundingReceipts{}
	now := time.Now()
	for _, receipt := range *receipts {
		if now.Before(receipt.FundedAt.Add(maxAge)) {
			pruned = append(pruned, receipt)
		}
	}
	*receipts = pruned
	// Save to db
	err := db.SaveFundingReceipts(receipts)
	if err != nil {
		// GCP will restart pod
		log.Fatal(err)
	}
}
*/

func (db Db) GetFundingReceipts(ctx context.Context) (FundingReceipts, error) {
	table := db.firestore.Collection("funding-receipts")
	docs, err := table.Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	var out FundingReceipts
	for _, snap := range docs {
		var receipt FundingReceipt

		err := snap.DataTo(&receipt)
		if err != nil {
			return nil, err
		}

		out = append(out, receipt)
	}

	return out, nil
}
