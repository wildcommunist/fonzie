package db

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"sync"
	"time"

	b64 "encoding/base64"
	"encoding/hex"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/option"
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
	ctx      context.Context
	receipts FundingReceipts
	rw       sync.RWMutex
}

func NewDb(ctx context.Context) *Db {

	return &Db{
		ctx:      ctx,
		receipts: FundingReceipts{},
	}
}

// ProvideFirestore returns a *firestore.Client
func initFirestore(ctx context.Context) (*firestore.Client, error) {
	var (
		app  *firebase.App
		json []byte
		err  error
	)
	conf := &firebase.Config{
		ProjectID:   os.Getenv("GCP_PROJECT"),
		DatabaseURL: os.Getenv("GCP_URL"),
	}
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
		app, err = firebase.NewApp(ctx, conf, option.WithoutAuthentication())
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

func (db *Db) SaveFundingReceipt(ctx context.Context, newReceipt FundingReceipt) error {
	// Safe receipt with timestamp etc

	db.rw.Lock()
	defer db.rw.Unlock()
	db.receipts = append(db.receipts, newReceipt)
	log.Infof("SAVED RECEIPT: %s %s(), %s", newReceipt.Username, newReceipt.ChainPrefix, newReceipt.FundedAt)
	fmt.Println(db.receipts)

	return nil
}

func (db *Db) PruneExpiredReceipts(ctx context.Context, beforeFundingTime time.Time) (int, error) {

	db.rw.Lock()
	defer db.rw.Unlock()

	receipts := FundingReceipts{}
	count := len(db.receipts)

	for _, v := range db.receipts {
		if v.FundedAt.After(beforeFundingTime) {
			receipts = append(receipts, v)
			//db.receipts = append(db.receipts[:k], db.receipts[k+1:]...)
		}
	}

	db.receipts = receipts

	// Delete stale receipts
	return count - len(db.receipts), nil
}

func (db *Db) GetFundingReceiptByUsernameAndChainPrefix(ctx context.Context, username string, chainPrefix string) (*FundingReceipt, error) {
	// Get the receipts

	db.rw.RLock()
	defer db.rw.RUnlock()

	for k, v := range db.receipts {
		if v.ChainPrefix == chainPrefix && v.Username == username {
			log.Infof("found: (%s)%s (%s)", chainPrefix, username, v.FundedAt)
			return &db.receipts[k], nil
		}
	}

	log.Infof("user: %s chain:%s was not found", username, chainPrefix)
	return nil, nil
}

func mkPKEY(username string, chainPrefix string) string {
	return getMD5Hash(username + chainPrefix)
}

func getMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
