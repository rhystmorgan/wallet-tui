package models

import "time"

type Asset struct {
	ID        string    `json:"id"`
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name"`
	Balance   string    `json:"balance"`
	Decimals  int       `json:"decimals"`
	Contract  string    `json:"contract,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AssetType int

const (
	AssetTypeVET AssetType = iota
	AssetTypeVTHO
	AssetTypeVIP180
)

func NewVETAsset(balance string) *Asset {
	return &Asset{
		ID:        "VET",
		Symbol:    "VET",
		Name:      "VeChain Token",
		Balance:   balance,
		Decimals:  18,
		UpdatedAt: time.Now(),
	}
}

func NewVTHOAsset(balance string) *Asset {
	return &Asset{
		ID:        "VTHO",
		Symbol:    "VTHO",
		Name:      "VeThor Token",
		Balance:   balance,
		Decimals:  18,
		UpdatedAt: time.Now(),
	}
}

func NewVIP180Asset(symbol, name, balance, contract string, decimals int) *Asset {
	return &Asset{
		ID:        contract,
		Symbol:    symbol,
		Name:      name,
		Balance:   balance,
		Decimals:  decimals,
		Contract:  contract,
		UpdatedAt: time.Now(),
	}
}
