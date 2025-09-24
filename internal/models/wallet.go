package models

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/darrenvechain/thorgo/crypto/hdwallet"
)

type Wallet struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	Mnemonic      string            `json:"-"`
	PrivateKey    *ecdsa.PrivateKey `json:"-"`
	CreatedAt     time.Time         `json:"created_at"`
	IsEncrypted   bool              `json:"is_encrypted"`
	CachedBalance *CachedBalance    `json:"-"`
	LastSync      time.Time         `json:"last_sync"`
}

type CachedBalance struct {
	VET         *big.Int
	VTHO        *big.Int
	LastUpdated time.Time
}

type WalletBalance struct {
	VET  string `json:"vet"`
	VTHO string `json:"vtho"`
}

type WalletInfo struct {
	Wallet  *Wallet        `json:"wallet"`
	Balance *WalletBalance `json:"balance"`
	Assets  []Asset        `json:"assets"`
}

func NewWallet(name, mnemonic string) (*Wallet, error) {
	derivationPath, err := hdwallet.ParseDerivationPath("m/44'/818'/0'/0/0")
	if err != nil {
		return nil, err
	}

	hdWallet, err := hdwallet.FromMnemonicAt(mnemonic, derivationPath)
	if err != nil {
		return nil, err
	}

	privateKey, err := hdWallet.PrivateKey()
	if err != nil {
		return nil, err
	}

	address := hdWallet.Address()

	return &Wallet{
		ID:         generateID(),
		Name:       name,
		Address:    address.Hex(),
		Mnemonic:   mnemonic,
		PrivateKey: privateKey,
		CreatedAt:  time.Now(),
	}, nil
}

func generateID() string {
	return time.Now().Format("20060102150405")
}

func (w *Wallet) SetBalance(vetBalance, vthoBalance *big.Int) {
	w.CachedBalance = &CachedBalance{
		VET:         new(big.Int).Set(vetBalance),
		VTHO:        new(big.Int).Set(vthoBalance),
		LastUpdated: time.Now(),
	}
	w.LastSync = time.Now()
}

func (w *Wallet) GetDisplayBalance() (string, string) {
	if w.CachedBalance == nil {
		return "0", "0"
	}

	// Convert from wei to VET/VTHO (divide by 10^18)
	vetWei := new(big.Float).SetInt(w.CachedBalance.VET)
	vthoWei := new(big.Float).SetInt(w.CachedBalance.VTHO)

	divisor := new(big.Float).SetFloat64(1e18)

	vetBalance := new(big.Float).Quo(vetWei, divisor)
	vthoBalance := new(big.Float).Quo(vthoWei, divisor)

	return fmt.Sprintf("%.4f", vetBalance), fmt.Sprintf("%.4f", vthoBalance)
}

func (w *Wallet) NeedsBalanceRefresh() bool {
	if w.CachedBalance == nil {
		return true
	}
	return time.Since(w.CachedBalance.LastUpdated) > 30*time.Second
}

func (w *Wallet) GetBalanceAge() time.Duration {
	if w.CachedBalance == nil {
		return time.Duration(0)
	}
	return time.Since(w.CachedBalance.LastUpdated)
}

func (w *Wallet) ClearBalance() {
	w.CachedBalance = nil
}
